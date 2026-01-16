package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	auditLogsKey = "audit:logs"
)

// Service represents the admin service
type Service struct {
	redis  *redis.Client
	search *SearchService // Milvus 向量搜索
}

// NewService creates a new admin service
func NewService(redisAddr, milvusAddr string) (*Service, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	ctx, cancel := NewContext()
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	// Initialize search service (optional, can be nil if Milvus unavailable)
	var searchSvc *SearchService
	if milvusAddr != "" {
		var err error
		searchSvc, err = NewSearchService(milvusAddr)
		if err != nil {
			// Log but don't fail - fallback to string matching
			fmt.Printf("⚠️  Milvus unavailable, using fallback search: %v\n", err)
		}
	}

	return &Service{
		redis:  client,
		search: searchSvc,
	}, nil
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

// userMatchesKeyword checks if a user matches the search keyword (case-insensitive)
func userMatchesKeyword(user User, keyword string) bool {
	if keyword == "" {
		return true
	}
	// Normalize: trim spaces and convert to lowercase
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	// Check all fields (case-insensitive for all)
	lowerID := strings.ToLower(user.ID)
	lowerName := strings.ToLower(user.Name)
	lowerEmail := strings.ToLower(user.Email)
	return strings.Contains(lowerName, keyword) ||
		strings.Contains(lowerID, keyword) ||
		strings.Contains(lowerEmail, keyword)
}

type CreateUserReq struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (s *Service) ListUsers(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := NewContext()
	defer cancel()

	keyword := r.URL.Query().Get("keyword")
	page, pageSize := parsePagination(r)

	var users []User

	// Use Milvus vector search if available and keyword provided
	if s.search != nil && keyword != "" {
		userIDs, err := s.search.SearchUsers(ctx, keyword, 100) // 搜索前 100 个
		if err == nil && len(userIDs) > 0 {
			// Fetch users by IDs
			for _, userID := range userIDs {
				data, err := s.redis.Get(ctx, "user:"+userID).Result()
				if err != nil {
					continue
				}
				var user User
				if err := json.Unmarshal([]byte(data), &user); err != nil {
					continue
				}
				users = append(users, user)
			}
		} else {
			// Fallback to string matching
			users = s.getUsersByStringMatch(ctx, keyword)
		}
	} else {
		// No keyword or Milvus unavailable - list all or use fallback
		users = s.getUsersByStringMatch(ctx, keyword)
	}

	// Pagination
	total := len(users)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	paginated := users[start:end]

	return s.respondJSON(w, 200, "success", map[string]interface{}{
		"data":       paginated,
		"page":       page,
		"page_size":  pageSize,
		"total":      total,
		"total_page": (total + pageSize - 1) / pageSize,
	})
}

// getUsersByStringMatch is the fallback search method
func (s *Service) getUsersByStringMatch(ctx context.Context, keyword string) []User {
	keys, err := s.redis.Keys(ctx, "user:*").Result()
	if err != nil {
		return nil
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
		if userMatchesKeyword(user, keyword) {
			users = append(users, user)
		}
	}
	return users
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

	// Index to Milvus for vector search
	if s.search != nil {
		if err := s.search.IndexUser(ctx, user); err != nil {
			// Log but don't fail
			fmt.Printf("⚠️  Failed to index user to Milvus: %v\n", err)
		}
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

	// Delete from Milvus
	if s.search != nil {
		if err := s.search.DeleteUser(ctx, req.ID); err != nil {
			fmt.Printf("⚠️  Failed to delete user from Milvus: %v\n", err)
		}
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

// documentMatchesKeyword checks if a document matches the search keyword
func documentMatchesKeyword(doc Document, keyword string) bool {
	if keyword == "" {
		return true
	}
	// Normalize: trim spaces and convert to lowercase
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	// Check all relevant fields (case-insensitive for all)
	lowerID := strings.ToLower(doc.ID)
	lowerTitle := strings.ToLower(doc.Title)
	lowerCategory := strings.ToLower(doc.Category)
	return strings.Contains(lowerTitle, keyword) ||
		strings.Contains(lowerCategory, keyword) ||
		strings.Contains(lowerID, keyword)
}

// parsePagination extracts and validates pagination parameters
func parsePagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 50
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}
	return page, pageSize
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

	keyword := r.URL.Query().Get("keyword")
	page, pageSize := parsePagination(r)

	var docs []Document

	// Use Milvus vector search if available and keyword provided
	if s.search != nil && keyword != "" {
		docIDs, err := s.search.SearchDocuments(ctx, keyword, 100)
		if err == nil && len(docIDs) > 0 {
			for _, docID := range docIDs {
				data, err := s.redis.Get(ctx, "doc:"+docID).Result()
				if err != nil {
					continue
				}
				var doc Document
				if err := json.Unmarshal([]byte(data), &doc); err != nil {
					continue
				}
				docs = append(docs, doc)
			}
		} else {
			docs = s.getDocsByStringMatch(ctx, keyword)
		}
	} else {
		docs = s.getDocsByStringMatch(ctx, keyword)
	}

	total := len(docs)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	paginated := docs[start:end]

	return s.respondJSON(w, 200, "success", map[string]interface{}{
		"data":       paginated,
		"page":       page,
		"page_size":  pageSize,
		"total":      total,
		"total_page": (total + pageSize - 1) / pageSize,
	})
}

// getDocsByStringMatch is the fallback search method
func (s *Service) getDocsByStringMatch(ctx context.Context, keyword string) []Document {
	keys, err := s.redis.Keys(ctx, "doc:*").Result()
	if err != nil {
		return nil
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
		if documentMatchesKeyword(doc, keyword) {
			docs = append(docs, doc)
		}
	}
	return docs
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

	// Index to Milvus
	if s.search != nil {
		if err := s.search.IndexDocument(ctx, doc); err != nil {
			fmt.Printf("⚠️  Failed to index document to Milvus: %v\n", err)
		}
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

	// Delete from Milvus
	if s.search != nil {
		if err := s.search.DeleteDocument(ctx, req.ID); err != nil {
			fmt.Printf("⚠️  Failed to delete document from Milvus: %v\n", err)
		}
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
	return s.redis.LPush(ctx, auditLogsKey, string(data)).Err()
}

func (s *Service) GetAuditLogs(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := NewContext()
	defer cancel()

	limit := 50 // default
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	// Get logs from Redis
	count, err := s.redis.LLen(ctx, auditLogsKey).Result()
	if err != nil {
		return err
	}

	// Fetch logs (newest first, limited)
	logStrs, err := s.redis.LRange(ctx, auditLogsKey, 0, int64(limit-1)).Result()
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
