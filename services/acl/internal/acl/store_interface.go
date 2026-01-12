package acl

import (
	"errors"
	"time"
)

type Visibility string

const (
	VisibilityPublic     Visibility = "public"
	VisibilityRestricted Visibility = "restricted"
)

var ErrInvalidVisibility = errors.New("invalid visibility")

// Store is a minimal ACL store interface.
// Implementations may be in-memory (local dev) or backed by Redis/DB.
//
// NOTE: This interface is used by HTTP handlers; keep it stable.
type Store interface {
	SetVisibility(tenantID, docID string, v Visibility) error
	Grant(tenantID, docID, userID string, validFrom time.Time, validTo *time.Time) error
	Revoke(tenantID, docID, userID string) error
	CheckBatch(tenantID, userID string, docIDs []string, now time.Time) ([]string, error)

	// ListGrants returns doc IDs that the user has explicit grants to.
	// It does NOT attempt to enumerate public docs.
	ListGrants(tenantID, userID string, now time.Time) []string

	// RevokeAllUser revokes all explicit grants for the user.
	RevokeAllUser(tenantID, userID string) error
}
