package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Service represents the admin service
type Service struct {
	redis *redis.Client
}

// NewService creates a new admin service
func NewService(redisAddr string) (*Service, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	ctx, cancel := NewContext()
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &Service{redis: client}, nil
}

// Response wrapper
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (s *Service) respondJSON(w http.ResponseWriter, code int, msg string, data interface{}) error {
	resp := Response{Code: code, Message: msg, Data: data}
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(resp)
}

// ============================================================
// Users
// ============================================================

// User represents a user in the system
type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type CreateUserReq struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Service) ListUsers(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := NewContext()
	defer cancel()

	keys, err := s.redis.Keys(ctx, "user:*").Result()
	if err != nil {
		return err
	}

	var users []User
	for _, key := range keys {
		data, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var user User
		if err := json.Unmarshal([]byte(data), &user); err != nil {
			continue
		}
		users = append(users, user)
	}

	return s.respondJSON(w, 200, "success", users)
}

func (s *Service) CreateUser(w http.ResponseWriter, r *http.Request) error {
	var req CreateUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	ctx, cancel := NewContext()
	defer cancel()

	// Check if user exists
	exists, err := s.redis.Exists(ctx, "user:"+req.ID).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return s.respondJSON(w, 400, "user already exists", nil)
	}

	// Create user
	now := time.Now().Format(time.RFC3339)
	user := User{
		ID:        req.ID,
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: now,
	}

	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	err = s.redis.Set(ctx, "user:"+req.ID, data, 0).Err()
	if err != nil {
		return err
	}

	// Audit log
	s.addAuditLog(ctx, "user_created", req.ID, "created user: "+req.Name)

	return s.respondJSON(w, 200, "user created", user)
}

type DeleteUserReq struct {
	ID string `json:"id"`
}

func (s *Service) DeleteUser(w http.ResponseWriter, r *http.Request) error {
	var req DeleteUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	ctx, cancel := NewContext()
	defer cancel()

	// Delete user
	err := s.redis.Del(ctx, "user:"+req.ID).Err()
	if err != nil {
		return err
	}

	// Delete user permissions
	tenants, err := s.redis.Keys(ctx, "acl:*:"+req.ID).Result()
	if err == nil {
		for _, key := range tenants {
			s.redis.Del(ctx, key)
		}
	}

	// Audit log
	s.addAuditLog(ctx, "user_deleted", req.ID, "deleted user: "+req.ID)

	return s.respondJSON(w, 200, "user deleted", nil)
}

// ============================================================
// Documents
// ============================================================

type Document struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	OwnerID   string `json:"owner_id"`
	CreatedAt string `json:"created_at"`
}

type CreateDocReq struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	OwnerID  string `json:"owner_id"`
}

func (s *Service) ListDocuments(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := NewContext()
	defer cancel()

	keys, err := s.redis.Keys(ctx, "doc:*").Result()
	if err != nil {
		return err
	}

	var docs []Document
	for _, key := range keys {
		data, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var doc Document
		if err := json.Unmarshal([]byte(data), &doc); err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return s.respondJSON(w, 200, "success", docs)
}

func (s *Service) CreateDocument(w http.ResponseWriter, r *http.Request) error {
	var req CreateDocReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	ctx, cancel := NewContext()
	defer cancel()

	// Check if document exists
	exists, err := s.redis.Exists(ctx, "doc:"+req.ID).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return s.respondJSON(w, 400, "document already exists", nil)
	}

	// Create document
	now := time.Now().Format(time.RFC3339)
	doc := Document{
		ID:        req.ID,
		Title:     req.Title,
		Category:  req.Category,
		OwnerID:   req.OwnerID,
		CreatedAt: now,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	err = s.redis.Set(ctx, "doc:"+req.ID, data, 0).Err()
	if err != nil {
		return err
	}

	// Audit log
	s.addAuditLog(ctx, "doc_created", req.ID, "created document: "+req.Title)

	return s.respondJSON(w, 200, "document created", doc)
}

type DeleteDocReq struct {
	ID string `json:"id"`
}

func (s *Service) DeleteDocument(w http.ResponseWriter, r *http.Request) error {
	var req DeleteDocReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	ctx, cancel := NewContext()
	defer cancel()

	// Delete document
	err := s.redis.Del(ctx, "doc:"+req.ID).Err()
	if err != nil {
		return err
	}

	// Audit log
	s.addAuditLog(ctx, "doc_deleted", req.ID, "deleted document: "+req.ID)

	return s.respondJSON(w, 200, "document deleted", nil)
}

// ============================================================
// Permissions
// ============================================================

type Permission struct {
	TenantID  string     `json:"tenant_id"`
	UserID    string     `json:"user_id"`
	DocIDs    []string   `json:"doc_ids"`
	Documents []Document `json:"documents"`
}

type GrantPermissionReq struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	DocID    string `json:"doc_id"`
}

func (s *Service) ListPermissions(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := NewContext()
	defer cancel()

	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		tenantID = "tenant-001" // default
	}

	keys, err := s.redis.Keys(ctx, "acl:"+tenantID+":*").Result()
	if err != nil {
		return err
	}

	var perms []Permission
	for _, key := range keys {
		docIDs, err := s.redis.SMembers(ctx, key).Result()
		if err != nil {
			continue
		}

		// Extract user_id from key (acl:tenant-001:user-001)
		parts := strings.Split(key, ":")
		if len(parts) != 3 {
			continue
		}

		// Fetch document details for all doc_ids
		var documents []Document
		for _, docID := range docIDs {
			docData, err := s.redis.Get(ctx, "doc:"+docID).Result()
			if err != nil {
				continue
			}
			var doc Document
			if err := json.Unmarshal([]byte(docData), &doc); err != nil {
				continue
			}
			documents = append(documents, doc)
		}

		perms = append(perms, Permission{
			TenantID:  parts[1],
			UserID:    parts[2],
			DocIDs:    docIDs,
			Documents: documents,
		})
	}

	return s.respondJSON(w, 200, "success", perms)
}

func (s *Service) GrantPermission(w http.ResponseWriter, r *http.Request) error {
	var req GrantPermissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	ctx, cancel := NewContext()
	defer cancel()

	// Add permission
	key := fmt.Sprintf("acl:%s:%s", req.TenantID, req.UserID)
	err := s.redis.SAdd(ctx, key, req.DocID).Err()
	if err != nil {
		return err
	}

	// Audit log
	s.addAuditLog(ctx, "permission_granted",
		fmt.Sprintf("%s:%s", req.UserID, req.DocID),
		fmt.Sprintf("granted %s access to %s", req.UserID, req.DocID))

	return s.respondJSON(w, 200, "permission granted", nil)
}

type RevokePermissionReq struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
	DocID    string `json:"doc_id"`
}

func (s *Service) RevokePermission(w http.ResponseWriter, r *http.Request) error {
	var req RevokePermissionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}

	ctx, cancel := NewContext()
	defer cancel()

	// Remove permission
	key := fmt.Sprintf("acl:%s:%s", req.TenantID, req.UserID)
	err := s.redis.SRem(ctx, key, req.DocID).Err()
	if err != nil {
		return err
	}

	// Audit log
	s.addAuditLog(ctx, "permission_revoked",
		fmt.Sprintf("%s:%s", req.UserID, req.DocID),
		fmt.Sprintf("revoked %s access to %s", req.UserID, req.DocID))

	return s.respondJSON(w, 200, "permission revoked", nil)
}

// ============================================================
// Audit Logs
// ============================================================

type AuditLog struct {
	ID        int64  `json:"id"`
	Action    string `json:"action"`
	Target    string `json:"target"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func (s *Service) addAuditLog(ctx context.Context, action, target, message string) error {
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	logEntry := AuditLog{
		Action:    action,
		Target:    target,
		Message:   message,
		Timestamp: timestamp,
	}
	data, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}
	return s.redis.LPush(ctx, "audit:logs", string(data)).Err()
}

func (s *Service) GetAuditLogs(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := NewContext()
	defer cancel()

	limit := 50 // default
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	// Get logs from Redis
	count, err := s.redis.LLen(ctx, "audit:logs").Result()
	if err != nil {
		return err
	}

	// Fetch logs (newest first, limited)
	logStrs, err := s.redis.LRange(ctx, "audit:logs", 0, int64(limit-1)).Result()
	if err != nil {
		return err
	}

	var logs []AuditLog
	for idx, logStr := range logStrs {
		var log AuditLog
		// Try to unmarshal as JSON
		if err := json.Unmarshal([]byte(logStr), &log); err != nil {
			// If JSON parsing fails, skip this log (old format)
			continue
		}
		log.ID = int64(idx + 1) // Simple ID assignment
		logs = append(logs, log)
	}

	return s.respondJSON(w, 200, "success", map[string]interface{}{
		"total": count,
		"limit": limit,
		"logs":  logs,
	})
}

// Helper
func NewContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
