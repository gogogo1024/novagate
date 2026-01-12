package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/gogogo1024/novagate/services/acl/internal/acl"
	"github.com/google/uuid"
)

var store acl.Store = acl.NewInMemoryStore()

const errInvalidJSON = "invalid json"
const errTenantUserUUID = "tenant_id and user_id must be uuid"

// SetStore swaps the underlying ACL store implementation.
//
// The default store is an in-memory implementation for local dev.
func SetStore(s acl.Store) {
	if s == nil {
		return
	}
	store = s
}

type checkBatchRequest struct {
	TenantID string   `json:"tenant_id"`
	UserID   string   `json:"user_id"`
	DocIDs   []string `json:"doc_ids"`
	Now      string   `json:"now,omitempty"` // RFC3339, optional (for debug/testing)
}

type checkBatchResponse struct {
	AllowedDocIDs []string `json:"allowed_doc_ids"`
}

type listGrantsResponse struct {
	GrantedDocIDs []string `json:"granted_doc_ids"`
}

// ACLCheckBatch POST /v1/acl/check-batch
func ACLCheckBatch(ctx context.Context, c *app.RequestContext) {
	var req checkBatchRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errInvalidJSON})
		return
	}
	if !isUUID(req.TenantID) || !isUUID(req.UserID) {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errTenantUserUUID})
		return
	}
	for _, id := range req.DocIDs {
		if !isUUID(id) {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "doc_ids must be uuid"})
			return
		}
	}

	now := time.Now()
	if req.Now != "" {
		t, err := time.Parse(time.RFC3339Nano, req.Now)
		if err != nil {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "now must be RFC3339"})
			return
		}
		now = t
	}

	// NOTE: Fail-closed on error to prevent leaking private content.
	allowed, err := store.CheckBatch(req.TenantID, req.UserID, req.DocIDs, now)
	if err != nil {
		// Store error: return empty list (fail-closed) rather than guessing.
		c.JSON(consts.StatusOK, checkBatchResponse{AllowedDocIDs: []string{}})
		return
	}
	c.JSON(consts.StatusOK, checkBatchResponse{AllowedDocIDs: allowed})
}

// ACLListGrants GET /v1/acl/grants?tenant_id=...&user_id=...&now=...
func ACLListGrants(ctx context.Context, c *app.RequestContext) {
	tenantID := string(c.Query("tenant_id"))
	userID := string(c.Query("user_id"))
	nowStr := string(c.Query("now"))

	if !isUUID(tenantID) || !isUUID(userID) {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errTenantUserUUID})
		return
	}

	now := time.Now()
	if nowStr != "" {
		t, err := time.Parse(time.RFC3339Nano, nowStr)
		if err != nil {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "now must be RFC3339"})
			return
		}
		now = t
	}

	granted := store.ListGrants(tenantID, userID, now)
	c.JSON(consts.StatusOK, listGrantsResponse{GrantedDocIDs: granted})
}

type grantRequest struct {
	TenantID   string `json:"tenant_id"`
	DocID      string `json:"doc_id"`
	UserID     string `json:"user_id"`
	ValidFrom  string `json:"valid_from,omitempty"` // RFC3339, optional
	ValidTo    string `json:"valid_to,omitempty"`   // RFC3339, optional; empty => permanent
	Restricted *bool  `json:"restricted,omitempty"` // optional: if true, set doc visibility to restricted
}

// ACLGrant POST /v1/acl/grant
func ACLGrant(ctx context.Context, c *app.RequestContext) {
	var req grantRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errInvalidJSON})
		return
	}
	if !isUUID(req.TenantID) || !isUUID(req.DocID) || !isUUID(req.UserID) {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "tenant_id/doc_id/user_id must be uuid"})
		return
	}

	validFrom := time.Now()
	if req.ValidFrom != "" {
		t, err := time.Parse(time.RFC3339Nano, req.ValidFrom)
		if err != nil {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "valid_from must be RFC3339"})
			return
		}
		validFrom = t
	}

	var validTo *time.Time
	if req.ValidTo != "" {
		t, err := time.Parse(time.RFC3339Nano, req.ValidTo)
		if err != nil {
			c.JSON(consts.StatusBadRequest, utils.H{"error": "valid_to must be RFC3339"})
			return
		}
		validTo = &t
	}

	// Optional: make doc restricted when granting.
	if req.Restricted != nil && *req.Restricted {
		if err := store.SetVisibility(req.TenantID, req.DocID, acl.VisibilityRestricted); err != nil {
			c.JSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
			return
		}
	}

	if err := store.Grant(req.TenantID, req.DocID, req.UserID, validFrom, validTo); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

type revokeRequest struct {
	TenantID string `json:"tenant_id"`
	DocID    string `json:"doc_id"`
	UserID   string `json:"user_id"`
}

type revokeAllUserRequest struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
}

// ACLRevoke POST /v1/acl/revoke
func ACLRevoke(ctx context.Context, c *app.RequestContext) {
	var req revokeRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errInvalidJSON})
		return
	}
	if !isUUID(req.TenantID) || !isUUID(req.DocID) || !isUUID(req.UserID) {
		c.JSON(consts.StatusBadRequest, utils.H{"error": "tenant_id/doc_id/user_id must be uuid"})
		return
	}

	if err := store.Revoke(req.TenantID, req.DocID, req.UserID); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// ACLRevokeAllUser POST /v1/acl/revoke-all
func ACLRevokeAllUser(ctx context.Context, c *app.RequestContext) {
	var req revokeAllUserRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errInvalidJSON})
		return
	}
	if !isUUID(req.TenantID) || !isUUID(req.UserID) {
		c.JSON(consts.StatusBadRequest, utils.H{"error": errTenantUserUUID})
		return
	}

	if err := store.RevokeAllUser(req.TenantID, req.UserID); err != nil {
		c.JSON(consts.StatusBadRequest, utils.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

func isUUID(v string) bool {
	_, err := uuid.Parse(v)
	return err == nil
}
