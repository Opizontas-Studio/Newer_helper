package server

import (
	"context"
	"log"
	"math"
	pb "newer_helper/grpc/proto/gen/punish"
	"newer_helper/model"
	punishments_db "newer_helper/utils/database/punishments"

	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PunishServer implements the PunishServer gRPC service
type PunishServer struct {
	pb.UnimplementedPunishServerServer
	punishDB *sqlx.DB
}

// NewPunishServer creates a new PunishServer instance
func NewPunishServer(punishDB *sqlx.DB) *PunishServer {
	return &PunishServer{
		punishDB: punishDB,
	}
}

// GetPunishStatus retrieves the current punishment status for a user
func (s *PunishServer) GetPunishStatus(ctx context.Context, req *pb.GetPunishStatusRequest) (*pb.GetPunishStatusResponse, error) {
	log.Printf("[PunishServer] GetPunishStatus called for user_id=%s, guild_id=%s", req.UserId, req.GuildId)

	if req.UserId == "" || req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and guild_id are required")
	}

	// Query for active punishments for this user in this guild
	query := `SELECT * FROM punishments
			  WHERE user_id = ?
			  AND guild_id = ?
			  AND punishment_status = 'active'
			  ORDER BY timestamp DESC`

	var records []model.PunishmentRecord
	err := s.punishDB.Select(&records, query, req.UserId, req.GuildId)
	if err != nil {
		log.Printf("[PunishServer] Error querying active punishments: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to query punishments: %v", err)
	}

	// Handle case where user has no punishments
	if len(records) == 0 {
		log.Printf("[PunishServer] No active punishments found for user %s in guild %s", req.UserId, req.GuildId)
		return &pb.GetPunishStatusResponse{
			Status:            "none",
			ActivePunishments: []*pb.Punishment{},
		}, nil
	}

	// Convert records to proto format
	protoPunishments := make([]*pb.Punishment, 0, len(records))
	for _, record := range records {
		protoPunishments = append(protoPunishments, convertToProtoPunishment(&record))
	}

	log.Printf("[PunishServer] Found %d active punishments for user %s", len(records), req.UserId)

	return &pb.GetPunishStatusResponse{
		Status:            "active",
		ActivePunishments: protoPunishments,
	}, nil
}

// GetPunishHistory retrieves the punishment history for a user with pagination
func (s *PunishServer) GetPunishHistory(ctx context.Context, req *pb.GetPunishHistoryRequest) (*pb.GetPunishHistoryResponse, error) {
	log.Printf("[PunishServer] GetPunishHistory called for user_id=%s, guild_id=%s, page=%d, page_size=%d",
		req.UserId, req.GuildId, req.Page, req.PageSize)

	if req.UserId == "" || req.GuildId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and guild_id are required")
	}

	// Set default page size if not provided or invalid
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10 // default page size
	}
	if pageSize > 100 {
		pageSize = 100 // max page size
	}

	// Set page to 1 if not provided or invalid
	page := req.Page
	if page <= 0 {
		page = 1
	}

	// Get all records for the user in this guild
	allRecords, err := punishments_db.GetPunishmentRecordsByUserID(s.punishDB, req.UserId, nil)
	if err != nil {
		log.Printf("[PunishServer] Error querying punishment history: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to query punishment history: %v", err)
	}

	// Filter by guild_id
	var guildRecords []model.PunishmentRecord
	for _, record := range allRecords {
		if record.GuildID == req.GuildId {
			guildRecords = append(guildRecords, record)
		}
	}

	totalRecords := len(guildRecords)

	// Handle case where user has no punishment history
	if totalRecords == 0 {
		log.Printf("[PunishServer] No punishment history found for user %s in guild %s", req.UserId, req.GuildId)
		return &pb.GetPunishHistoryResponse{
			Punishments:  []*pb.Punishment{},
			TotalRecords: 0,
			TotalPages:   0,
			CurrentPage:  page,
		}, nil
	}

	// Calculate pagination
	totalPages := int32(math.Ceil(float64(totalRecords) / float64(pageSize)))
	offset := (page - 1) * pageSize

	// Handle case where page is out of range
	if offset >= int32(totalRecords) {
		log.Printf("[PunishServer] Page %d is out of range (total pages: %d)", page, totalPages)
		return &pb.GetPunishHistoryResponse{
			Punishments:  []*pb.Punishment{},
			TotalRecords: int32(totalRecords),
			TotalPages:   totalPages,
			CurrentPage:  page,
		}, nil
	}

	// Get the records for this page
	end := offset + pageSize
	if end > int32(totalRecords) {
		end = int32(totalRecords)
	}
	pageRecords := guildRecords[offset:end]

	// Convert records to proto format
	protoPunishments := make([]*pb.Punishment, 0, len(pageRecords))
	for _, record := range pageRecords {
		protoPunishments = append(protoPunishments, convertToProtoPunishment(&record))
	}

	log.Printf("[PunishServer] Returning page %d/%d with %d records (total: %d)",
		page, totalPages, len(pageRecords), totalRecords)

	return &pb.GetPunishHistoryResponse{
		Punishments:  protoPunishments,
		TotalRecords: int32(totalRecords),
		TotalPages:   totalPages,
		CurrentPage:  page,
	}, nil
}

// convertToProtoPunishment converts a model.PunishmentRecord to a proto Punishment
func convertToProtoPunishment(record *model.PunishmentRecord) *pb.Punishment {
	return &pb.Punishment{
		PunishmentId:     record.PunishmentID,
		MessageId:        record.MessageID,
		AdminId:          record.AdminID,
		UserId:           record.UserID,
		UserUsername:     record.UserUsername,
		Reason:           record.Reason,
		GuildId:          record.GuildID,
		Timestamp:        record.Timestamp,
		Evidence:         record.Evidence,
		ActionType:       record.ActionType,
		TempRolesJson:    record.TempRolesJSON,
		RolesRemoveAt:    record.RolesRemoveAt,
		PunishmentStatus: record.PunishmentStatus,
	}
}
