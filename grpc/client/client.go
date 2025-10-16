package client

import (
	"context"
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
	}, nil
}

// Connect establishes connection to the gateway server
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Create connection to the server
	conn, err := grpc.NewClient(
		c.serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
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
				Services: []string{}, // Empty for now - will add services later
			},
		},
	}

	if err := stream.Send(registerMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send registration: %w", err)
	}

	log.Printf("Connected to gateway at %s as %s", c.serverAddr, c.clientName)

	// Start background goroutines
	go c.receiveLoop()
	go c.heartbeatLoop()

	return nil
}

// receiveLoop handles incoming messages from the gateway
func (c *Client) receiveLoop() {
	defer func() {
		close(c.done)
	}()

	for {
		msg, err := c.stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Println("Gateway closed the connection")
				return
			}
			log.Printf("Error receiving message: %v", err)
			return
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
			return
		case <-ticker.C:
			c.mu.Lock()
			connID := c.connectionID
			c.mu.Unlock()

			// Only send heartbeat if we have a connection ID
			if connID == "" {
				log.Printf("Skipping heartbeat - no connection_id yet")
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
