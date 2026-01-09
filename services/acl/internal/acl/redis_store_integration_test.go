package acl

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisStore_Integration_RevokeCleansKeys(t *testing.T) {
	addr := os.Getenv("ACL_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set ACL_TEST_REDIS_ADDR to run Redis integration tests")
	}

	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	prefix := "acltest:" + time.Now().UTC().Format("20060102T150405.000000000") + ":"
	s := NewRedisStore(rdb, prefix)

	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	docID := "33333333-3333-3333-3333-333333333333"

	if err := s.SetVisibility(tenantID, docID, VisibilityRestricted); err != nil {
		t.Fatalf("SetVisibility: %v", err)
	}

	now := time.Now()
	if err := s.Grant(tenantID, docID, userID, now, nil); err != nil {
		t.Fatalf("Grant(permanent): %v", err)
	}

	// Keys should exist after grant.
	docPermKey := s.keyPermanent(tenantID, docID)
	userPermKey := s.keyUserPermanent(tenantID, userID)
	if n, err := rdb.Exists(ctx, docPermKey, userPermKey).Result(); err != nil {
		t.Fatalf("exists(after grant): %v", err)
	} else if n != 2 {
		t.Fatalf("expected 2 keys to exist after grant, got=%d", n)
	}

	if err := s.Revoke(tenantID, docID, userID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Keys should be deleted once empty.
	if n, err := rdb.Exists(ctx, docPermKey, userPermKey).Result(); err != nil {
		t.Fatalf("exists(after revoke): %v", err)
	} else if n != 0 {
		t.Fatalf("expected keys to be deleted after revoke, got=%d", n)
	}
}
