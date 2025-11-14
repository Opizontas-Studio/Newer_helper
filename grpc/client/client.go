package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	punishpb "newer_helper/grpc/proto/gen/punish"
	proto "newer_helper/grpc/proto/gen/registry"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

// Client represents a gRPC client that connects to the gateway
type Client struct {
	serverAddr   string
	clientName   string
	token        string
	connectionID string // Connection ID from gateway
	conn         *grpc.ClientConn
	stream       proto.RegistryService_EstablishConnectionClient
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	punishServer punishpb.PunishServerServer // Punish service handler

	// Reconnection settings
	reconnecting        bool
	maxReconnectAttempts int
	reconnectDelay      time.Duration
	maxReconnectDelay   time.Duration
}

// NewClient creates a new gRPC client instance
func NewClient(punishServer punishpb.PunishServerServer) (*Client, error) {
	serverAddr := os.Getenv("GRPC_SERVER_ADDRESS")
	clientName := os.Getenv("GRPC_CLIENT_NAME")
	token := os.Getenv("GRPC_TOKEN")

	if serverAddr == "" || clientName == "" || token == "" {
		return nil, fmt.Errorf("missing required environment variables: GRPC_SERVER_ADDRESS, GRPC_CLIENT_NAME, or GRPC_TOKEN")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		serverAddr:   serverAddr,
		clientName:   clientName,
		token:        token,
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
		punishServer: punishServer,

		// Initialize reconnection settings
		maxReconnectAttempts: 10,
		reconnectDelay:      2 * time.Second,
		maxReconnectDelay:   60 * time.Second,
	}, nil
}

// Connect establishes connection to the gateway server
func (c *Client) Connect() error {
	c.mu.Lock()
	err := c.doConnect()
	c.mu.Unlock()

	if err != nil {
		return err
	}

	// Start background goroutines
	go c.receiveLoop()
	go c.heartbeatLoop()

	return nil
}

// doConnect performs the actual connection logic (must be called with lock held)
func (c *Client) doConnect() error {
	// Remove https:// or http:// prefix if present
	serverAddr := strings.TrimPrefix(c.serverAddr, "https://")
	serverAddr = strings.TrimPrefix(serverAddr, "http://")

	// Try TLS connection first
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false, // Set to true if using self-signed certificates
	}

	conn, err := grpc.NewClient(
		serverAddr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)

	if err != nil {
		log.Printf("TLS connection failed: %v, trying insecure connection...", err)

		// Fallback to insecure connection
		conn, err = grpc.NewClient(
			serverAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to server (both TLS and insecure): %w", err)
		}
		log.Printf("Connected using insecure connection (no TLS)")
	} else {
		log.Printf("Connected using TLS")
	}

	c.conn = conn

	// Create registry service client
	registryClient := proto.NewRegistryServiceClient(conn)

	// Establish bidirectional stream
	stream, err := registryClient.EstablishConnection(c.ctx)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to establish connection stream: %w", err)
	}
	c.stream = stream

	// Send registration message
	registerMsg := &proto.ConnectionMessage{
		MessageType: &proto.ConnectionMessage_Register{
			Register: &proto.ConnectionRegister{
				ApiKey:   c.token,
				Services: []string{fmt.Sprintf("%s.punish", c.clientName)},
			},
		},
	}

	if err := stream.Send(registerMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send registration: %w", err)
	}

	log.Printf("Connected to gateway at %s as %s", c.serverAddr, c.clientName)

	return nil
}

// reconnect attempts to reconnect to the gateway with exponential backoff
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		// Already reconnecting, don't start another attempt
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	log.Println("Starting reconnection process...")

	delay := c.reconnectDelay
	for attempt := 1; attempt <= c.maxReconnectAttempts; attempt++ {
		// Check if context was cancelled (user called Close())
		select {
		case <-c.ctx.Done():
			log.Println("Reconnection cancelled by user")
			return
		default:
		}

		log.Printf("Reconnection attempt %d/%d (waiting %v)...", attempt, c.maxReconnectAttempts, delay)
		time.Sleep(delay)

		// Clean up old connection
		c.mu.Lock()
		if c.stream != nil {
			c.stream.CloseSend()
			c.stream = nil
		}
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.connectionID = ""

		// Attempt to reconnect
		err := c.doConnect()
		c.mu.Unlock()

		if err != nil {
			log.Printf("Reconnection attempt %d failed: %v", attempt, err)
			// Exponential backoff: double the delay, up to max
			delay = delay * 2
			if delay > c.maxReconnectDelay {
				delay = c.maxReconnectDelay
			}
			continue
		}

		// Success! Restart background goroutines
		log.Printf("Reconnection successful on attempt %d", attempt)
		go c.receiveLoop()
		go c.heartbeatLoop()
		return
	}

	// Max attempts reached
	log.Printf("Failed to reconnect after %d attempts. Giving up.", c.maxReconnectAttempts)
	close(c.done)
}

// receiveLoop handles incoming messages from the gateway
func (c *Client) receiveLoop() {
	for {
		// Check if we should exit (user called Close())
		select {
		case <-c.ctx.Done():
			log.Println("receiveLoop: context cancelled, exiting")
			close(c.done)
			return
		default:
		}

		msg, err := c.stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Println("Gateway closed the connection (EOF)")
			} else {
				log.Printf("Error receiving message: %v", err)
			}

			// Check if context is cancelled before attempting reconnect
			select {
			case <-c.ctx.Done():
				log.Println("receiveLoop: context cancelled, not reconnecting")
				close(c.done)
				return
			default:
				// Trigger reconnection and exit this loop
				// A new receiveLoop will be started upon successful reconnect
				log.Println("receiveLoop: triggering reconnection...")
				go c.reconnect()
				return
			}
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes incoming messages from the gateway
func (c *Client) handleMessage(msg *proto.ConnectionMessage) {
	switch m := msg.MessageType.(type) {
	case *proto.ConnectionMessage_Request:
		log.Printf("Received request: %s (method: %s)", m.Request.RequestId, m.Request.MethodPath)

		// Route to appropriate service handler
		c.handleServiceRequest(m.Request)

	case *proto.ConnectionMessage_Status:
		log.Printf("Received status: %v - %s", m.Status.Status, m.Status.Message)

		// Extract and store connection ID if connected
		if m.Status.Status == proto.ConnectionStatus_CONNECTED && m.Status.ConnectionId != "" {
			c.mu.Lock()
			c.connectionID = m.Status.ConnectionId
			c.mu.Unlock()
			log.Printf("Stored connection_id: %s", c.connectionID)
		}

	case *proto.ConnectionMessage_Heartbeat:
		// Gateway heartbeat received, no action needed
		log.Printf("Received heartbeat from gateway")

	case *proto.ConnectionMessage_Event:
		// TODO: Handle events
		log.Printf("Received event: %s (type: %s)", m.Event.EventId, m.Event.EventType)

	default:
		log.Printf("Received unknown message type")
	}
}

// handleServiceRequest routes requests to appropriate service handlers
func (c *Client) handleServiceRequest(req *proto.ForwardRequest) {
	var response *proto.ConnectionMessage

	// Parse method path: expected format is "/{client_name}.{package}/{Method}"
	// Example: "/discord_bot.punish/GetPunishStatus"
	// where client_name is configured in the gateway, and package is from the proto file
	log.Printf("Routing request with method path: %s", req.MethodPath)

	// Route based on method path suffix (after the package name)
	// We check if the path ends with our service methods
	switch {
	case strings.HasSuffix(req.MethodPath, "/GetPunishStatus"):
		response = c.handleGetPunishStatus(req)
	case strings.HasSuffix(req.MethodPath, "/GetPunishHistory"):
		response = c.handleGetPunishHistory(req)
	default:
		log.Printf("Unknown method path: %s", req.MethodPath)
		response = &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   404,
					ErrorMessage: fmt.Sprintf("method not found: %s", req.MethodPath),
				},
			},
		}
	}

	// Send response back through the stream
	c.mu.Lock()
	err := c.stream.Send(response)
	c.mu.Unlock()

	if err != nil {
		log.Printf("Failed to send response for request %s: %v", req.RequestId, err)
	}
}

// handleGetPunishStatus handles GetPunishStatus requests
func (c *Client) handleGetPunishStatus(req *proto.ForwardRequest) *proto.ConnectionMessage {
	// Unmarshal request body
	var punishReq punishpb.GetPunishStatusRequest
	if err := json.Unmarshal(req.Payload, &punishReq); err != nil {
		log.Printf("Failed to unmarshal GetPunishStatus request: %v", err)
		return &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   400,
					ErrorMessage: fmt.Sprintf("invalid request body: %v", err),
				},
			},
		}
	}

	// Call the service handler
	resp, err := c.punishServer.GetPunishStatus(context.Background(), &punishReq)
	if err != nil {
		log.Printf("GetPunishStatus failed: %v", err)
		return &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   500,
					ErrorMessage: fmt.Sprintf("service error: %v", err),
				},
			},
		}
	}

	// Marshal response
	respBody, err := protojson.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal GetPunishStatus response: %v", err)
		return &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   500,
					ErrorMessage: fmt.Sprintf("failed to marshal response: %v", err),
				},
			},
		}
	}

	return &proto.ConnectionMessage{
		MessageType: &proto.ConnectionMessage_Response{
			Response: &proto.ForwardResponse{
				RequestId:  req.RequestId,
				StatusCode: 200,
				Payload:    respBody,
			},
		},
	}
}

// handleGetPunishHistory handles GetPunishHistory requests
func (c *Client) handleGetPunishHistory(req *proto.ForwardRequest) *proto.ConnectionMessage {
	// Unmarshal request body
	var punishReq punishpb.GetPunishHistoryRequest
	if err := json.Unmarshal(req.Payload, &punishReq); err != nil {
		log.Printf("Failed to unmarshal GetPunishHistory request: %v", err)
		return &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   400,
					ErrorMessage: fmt.Sprintf("invalid request body: %v", err),
				},
			},
		}
	}

	// Call the service handler
	resp, err := c.punishServer.GetPunishHistory(context.Background(), &punishReq)
	if err != nil {
		log.Printf("GetPunishHistory failed: %v", err)
		return &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   500,
					ErrorMessage: fmt.Sprintf("service error: %v", err),
				},
			},
		}
	}

	// Marshal response
	respBody, err := protojson.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal GetPunishHistory response: %v", err)
		return &proto.ConnectionMessage{
			MessageType: &proto.ConnectionMessage_Response{
				Response: &proto.ForwardResponse{
					RequestId:    req.RequestId,
					StatusCode:   500,
					ErrorMessage: fmt.Sprintf("failed to marshal response: %v", err),
				},
			},
		}
	}

	return &proto.ConnectionMessage{
		MessageType: &proto.ConnectionMessage_Response{
			Response: &proto.ForwardResponse{
				RequestId:  req.RequestId,
				StatusCode: 200,
				Payload:    respBody,
			},
		},
	}
}

// heartbeatLoop sends periodic heartbeat messages to the gateway
func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Println("heartbeatLoop: context cancelled, exiting")
			return
		case <-ticker.C:
			c.mu.Lock()
			connID := c.connectionID
			stream := c.stream
			c.mu.Unlock()

			// Skip heartbeat if connection is not ready
			if connID == "" {
				log.Printf("Skipping heartbeat - no connection_id yet")
				continue
			}

			if stream == nil {
				log.Printf("Skipping heartbeat - stream not available (reconnecting?)")
				continue
			}

			heartbeat := &proto.ConnectionMessage{
				MessageType: &proto.ConnectionMessage_Heartbeat{
					Heartbeat: &proto.Heartbeat{
						Timestamp:    time.Now().Unix(),
						ConnectionId: connID, // Use connection_id from gateway
					},
				},
			}

			c.mu.Lock()
			err := c.stream.Send(heartbeat)
			c.mu.Unlock()

			if err != nil {
				log.Printf("Failed to send heartbeat: %v", err)
				log.Println("heartbeatLoop: exiting due to send error")
				return
			}
			log.Printf("Sent heartbeat to gateway with connection_id: %s", connID)
		}
	}
}

// Close gracefully shuts down the client connection
func (c *Client) Close() error {
	c.cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stream != nil {
		if err := c.stream.CloseSend(); err != nil {
			log.Printf("Error closing stream: %v", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
			return err
		}
	}

	// Wait for receive loop to finish
	<-c.done

	log.Println("gRPC client closed")
	return nil
}
