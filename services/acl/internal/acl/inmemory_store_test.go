package acl

import (
	"testing"
	"time"
)

func TestInMemoryStore_DefaultPublic_AllowsWithoutVisibilityKey(t *testing.T) {
	s := NewInMemoryStore()

	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	docID := "33333333-3333-3333-3333-333333333333"

	allowed, _ := s.CheckBatch(tenantID, userID, []string{docID}, time.Now())
	if len(allowed) != 1 || allowed[0] != docID {
		t.Fatalf("expected doc to be allowed by default, got=%v", allowed)
	}
}

func TestInMemoryStore_RestrictedRequiresGrant_RevokeWorks(t *testing.T) {
	s := NewInMemoryStore()

	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	docID := "33333333-3333-3333-3333-333333333333"

	if err := s.SetVisibility(tenantID, docID, VisibilityRestricted); err != nil {
		t.Fatalf("SetVisibility: %v", err)
	}

	now := time.Now()
	allowed, _ := s.CheckBatch(tenantID, userID, []string{docID}, now)
	if len(allowed) != 0 {
		t.Fatalf("expected restricted doc to be denied without grant, got=%v", allowed)
	}

	if err := s.Grant(tenantID, docID, userID, now, nil); err != nil {
		t.Fatalf("Grant(permanent): %v", err)
	}
	allowed, _ = s.CheckBatch(tenantID, userID, []string{docID}, now)
	if len(allowed) != 1 || allowed[0] != docID {
		t.Fatalf("expected restricted doc to be allowed with permanent grant, got=%v", allowed)
	}

	if err := s.Revoke(tenantID, docID, userID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	allowed, _ = s.CheckBatch(tenantID, userID, []string{docID}, now)
	if len(allowed) != 0 {
		t.Fatalf("expected revoked restricted doc to be denied, got=%v", allowed)
	}
}

func TestInMemoryStore_ExpiringGrant_RespectsNow(t *testing.T) {
	s := NewInMemoryStore()

	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	docID := "33333333-3333-3333-3333-333333333333"

	if err := s.SetVisibility(tenantID, docID, VisibilityRestricted); err != nil {
		t.Fatalf("SetVisibility: %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	validFrom := base
	validTo := base.Add(10 * time.Minute)

	if err := s.Grant(tenantID, docID, userID, validFrom, &validTo); err != nil {
		t.Fatalf("Grant(expiring): %v", err)
	}

	allowed, _ := s.CheckBatch(tenantID, userID, []string{docID}, base.Add(5*time.Minute))
	if len(allowed) != 1 {
		t.Fatalf("expected allowed during validity, got=%v", allowed)
	}
	allowed, _ = s.CheckBatch(tenantID, userID, []string{docID}, base.Add(11*time.Minute))
	if len(allowed) != 0 {
		t.Fatalf("expected denied after expiration, got=%v", allowed)
	}

	grants := s.ListGrants(tenantID, userID, base.Add(5*time.Minute))
	if len(grants) != 1 || grants[0] != docID {
		t.Fatalf("expected ListGrants to include doc during validity, got=%v", grants)
	}
	grants = s.ListGrants(tenantID, userID, base.Add(11*time.Minute))
	if len(grants) != 0 {
		t.Fatalf("expected ListGrants to exclude doc after expiration, got=%v", grants)
	}
}

func TestInMemoryStore_Grant_ValidToBeforeValidFromErrors(t *testing.T) {
	s := NewInMemoryStore()
	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	docID := "33333333-3333-3333-3333-333333333333"

	validFrom := time.Date(2026, 1, 1, 0, 0, 10, 0, time.UTC)
	validTo := time.Date(2026, 1, 1, 0, 0, 9, 0, time.UTC)
	if err := s.Grant(tenantID, docID, userID, validFrom, &validTo); err == nil {
		t.Fatalf("expected error when valid_to < valid_from")
	}
}

func TestInMemoryStore_RevokeAllUser_RemovesBothEdgesAndCleansUp(t *testing.T) {
	s := NewInMemoryStore()

	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	docPerm := "33333333-3333-3333-3333-333333333333"
	docExp := "44444444-4444-4444-4444-444444444444"

	if err := s.SetVisibility(tenantID, docPerm, VisibilityRestricted); err != nil {
		t.Fatalf("SetVisibility(docPerm): %v", err)
	}
	if err := s.SetVisibility(tenantID, docExp, VisibilityRestricted); err != nil {
		t.Fatalf("SetVisibility(docExp): %v", err)
	}

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.Grant(tenantID, docPerm, userID, now, nil); err != nil {
		t.Fatalf("Grant(permanent): %v", err)
	}
	vt := now.Add(10 * time.Minute)
	if err := s.Grant(tenantID, docExp, userID, now, &vt); err != nil {
		t.Fatalf("Grant(expiring): %v", err)
	}

	grants := s.ListGrants(tenantID, userID, now)
	if len(grants) != 2 {
		t.Fatalf("expected 2 grants before revoke-all, got=%v", grants)
	}

	if err := s.RevokeAllUser(tenantID, userID); err != nil {
		t.Fatalf("RevokeAllUser: %v", err)
	}

	grants = s.ListGrants(tenantID, userID, now)
	if len(grants) != 0 {
		t.Fatalf("expected 0 grants after revoke-all, got=%v", grants)
	}

	allowed, _ := s.CheckBatch(tenantID, userID, []string{docPerm, docExp}, now)
	if len(allowed) != 0 {
		t.Fatalf("expected all restricted docs denied after revoke-all, got=%v", allowed)
	}

	st := s.Stats()
	if st.PermanentDocEdges != 0 || st.ExpiringDocEdges != 0 {
		t.Fatalf("expected edges to be 0 after revoke-all, got=%+v", st)
	}
}
