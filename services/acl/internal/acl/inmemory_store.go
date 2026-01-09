package acl

import (
	"errors"
	"sync"
	"time"
)

type InMemoryStore struct {
	mu sync.RWMutex

	// tenant -> doc -> visibility
	visibility map[string]map[string]Visibility

	// tenant -> doc -> user -> struct{}
	permanent map[string]map[string]map[string]struct{}

	// tenant -> doc -> user -> validTo
	expiring map[string]map[string]map[string]time.Time
}

var _ Store = (*InMemoryStore)(nil)

type InMemoryStats struct {
	Tenants           int `json:"tenants"`
	VisibilityDocs    int `json:"visibility_docs"`
	PermanentDocEdges int `json:"permanent_doc_edges"`
	ExpiringDocEdges  int `json:"expiring_doc_edges"`
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		visibility: make(map[string]map[string]Visibility),
		permanent:  make(map[string]map[string]map[string]struct{}),
		expiring:   make(map[string]map[string]map[string]time.Time),
	}
}

func (s *InMemoryStore) SetVisibility(tenantID, docID string, v Visibility) error {
	if v != VisibilityPublic && v != VisibilityRestricted {
		return ErrInvalidVisibility
	}
	if tenantID == "" || docID == "" {
		return errors.New("tenant_id/doc_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.visibility[tenantID]
	if m == nil {
		m = make(map[string]Visibility)
		s.visibility[tenantID] = m
	}
	m[docID] = v
	return nil
}

func (s *InMemoryStore) Grant(tenantID, docID, userID string, validFrom time.Time, validTo *time.Time) error {
	if err := validateGrantArgs(tenantID, docID, userID, validFrom, validTo); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if validTo == nil {
		s.grantPermanentLocked(tenantID, docID, userID)
		return nil
	}

	s.grantExpiringLocked(tenantID, docID, userID, *validTo)
	return nil
}

func (s *InMemoryStore) Revoke(tenantID, docID, userID string) error {
	if tenantID == "" || docID == "" || userID == "" {
		return errors.New("tenant_id/doc_id/user_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if docs := s.permanent[tenantID]; docs != nil {
		if users := docs[docID]; users != nil {
			delete(users, userID)
			if len(users) == 0 {
				delete(docs, docID)
			}
		}
		if len(docs) == 0 {
			delete(s.permanent, tenantID)
		}
	}
	if docs := s.expiring[tenantID]; docs != nil {
		if users := docs[docID]; users != nil {
			delete(users, userID)
			if len(users) == 0 {
				delete(docs, docID)
			}
		}
		if len(docs) == 0 {
			delete(s.expiring, tenantID)
		}
	}
	return nil
}

func (s *InMemoryStore) CheckBatch(tenantID, userID string, docIDs []string, now time.Time) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tenantID == "" || userID == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	allowed := make([]string, 0, len(docIDs))
	for _, docID := range docIDs {
		if docID == "" {
			continue
		}
		if s.visibilityOfLocked(tenantID, docID) == VisibilityPublic {
			allowed = append(allowed, docID)
			continue
		}
		if s.hasPermanentGrantLocked(tenantID, docID, userID) {
			allowed = append(allowed, docID)
			continue
		}
		if s.hasExpiringGrantLocked(tenantID, docID, userID, now) {
			allowed = append(allowed, docID)
			continue
		}
	}
	return allowed
}

func (s *InMemoryStore) ListGrants(tenantID, userID string, now time.Time) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tenantID == "" || userID == "" {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	seen := make(map[string]struct{})
	s.addPermanentGrantsLocked(tenantID, userID, seen)
	s.addExpiringGrantsLocked(tenantID, userID, now, seen)

	out := make([]string, 0, len(seen))
	for docID := range seen {
		out = append(out, docID)
	}
	return out
}

func (s *InMemoryStore) addPermanentGrantsLocked(tenantID, userID string, seen map[string]struct{}) {
	if seen == nil {
		return
	}
	if docs := s.permanent[tenantID]; docs != nil {
		for docID, users := range docs {
			if docID == "" || users == nil {
				continue
			}
			if _, ok := users[userID]; ok {
				seen[docID] = struct{}{}
			}
		}
	}
}

func (s *InMemoryStore) addExpiringGrantsLocked(tenantID, userID string, now time.Time, seen map[string]struct{}) {
	if seen == nil {
		return
	}
	if docs := s.expiring[tenantID]; docs != nil {
		for docID, users := range docs {
			if docID == "" || users == nil {
				continue
			}
			if validTo, ok := users[userID]; ok {
				if now.Before(validTo) {
					seen[docID] = struct{}{}
				}
			}
		}
	}
}

func (s *InMemoryStore) RevokeAllUser(tenantID, userID string) error {
	if tenantID == "" || userID == "" {
		return errors.New("tenant_id/user_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if docs := s.permanent[tenantID]; docs != nil {
		for docID, users := range docs {
			if users == nil {
				continue
			}
			delete(users, userID)
			if len(users) == 0 {
				delete(docs, docID)
			}
		}
		if len(docs) == 0 {
			delete(s.permanent, tenantID)
		}
	}
	if docs := s.expiring[tenantID]; docs != nil {
		for docID, users := range docs {
			if users == nil {
				continue
			}
			delete(users, userID)
			if len(users) == 0 {
				delete(docs, docID)
			}
		}
		if len(docs) == 0 {
			delete(s.expiring, tenantID)
		}
	}
	return nil
}

func (s *InMemoryStore) Stats() InMemoryStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	st := InMemoryStats{}
	st.Tenants = len(s.visibility)

	for _, docs := range s.visibility {
		st.VisibilityDocs += len(docs)
	}
	for _, docs := range s.permanent {
		for _, users := range docs {
			st.PermanentDocEdges += len(users)
		}
	}
	for _, docs := range s.expiring {
		for _, users := range docs {
			st.ExpiringDocEdges += len(users)
		}
	}
	return st
}

func validateGrantArgs(tenantID, docID, userID string, validFrom time.Time, validTo *time.Time) error {
	if tenantID == "" || docID == "" || userID == "" {
		return errors.New("tenant_id/doc_id/user_id is required")
	}
	if !validFrom.IsZero() && validTo != nil && !validTo.IsZero() {
		if validTo.Before(validFrom) {
			return errors.New("valid_to must be >= valid_from")
		}
	}
	return nil
}

func (s *InMemoryStore) grantPermanentLocked(tenantID, docID, userID string) {
	docs := s.permanent[tenantID]
	if docs == nil {
		docs = make(map[string]map[string]struct{})
		s.permanent[tenantID] = docs
	}
	users := docs[docID]
	if users == nil {
		users = make(map[string]struct{})
		docs[docID] = users
	}
	users[userID] = struct{}{}

	// Remove any expiring record.
	if docs2 := s.expiring[tenantID]; docs2 != nil {
		if users2 := docs2[docID]; users2 != nil {
			delete(users2, userID)
			if len(users2) == 0 {
				delete(docs2, docID)
			}
		}
		if len(docs2) == 0 {
			delete(s.expiring, tenantID)
		}
	}
}

func (s *InMemoryStore) grantExpiringLocked(tenantID, docID, userID string, validTo time.Time) {
	docs := s.expiring[tenantID]
	if docs == nil {
		docs = make(map[string]map[string]time.Time)
		s.expiring[tenantID] = docs
	}
	users := docs[docID]
	if users == nil {
		users = make(map[string]time.Time)
		docs[docID] = users
	}
	users[userID] = validTo

	// Remove any permanent record.
	if docs2 := s.permanent[tenantID]; docs2 != nil {
		if users2 := docs2[docID]; users2 != nil {
			delete(users2, userID)
			if len(users2) == 0 {
				delete(docs2, docID)
			}
		}
		if len(docs2) == 0 {
			delete(s.permanent, tenantID)
		}
	}
}

func (s *InMemoryStore) visibilityOfLocked(tenantID, docID string) Visibility {
	if m := s.visibility[tenantID]; m != nil {
		if vv, ok := m[docID]; ok {
			return vv
		}
	}
	return VisibilityPublic
}

func (s *InMemoryStore) hasPermanentGrantLocked(tenantID, docID, userID string) bool {
	if docs := s.permanent[tenantID]; docs != nil {
		if users := docs[docID]; users != nil {
			_, ok := users[userID]
			return ok
		}
	}
	return false
}

func (s *InMemoryStore) hasExpiringGrantLocked(tenantID, docID, userID string, now time.Time) bool {
	if docs := s.expiring[tenantID]; docs != nil {
		if users := docs[docID]; users != nil {
			if validTo, ok := users[userID]; ok {
				return now.Before(validTo)
			}
		}
	}
	return false
}
