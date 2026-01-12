package novagate

import (
	"testing"
	"time"
)

func TestConnContext_Reserve_TrackBufferUsage(t *testing.T) {
	ctx := NewConnContext()

	maxBuf := int64(256 * 1024) // 262144 bytes

	// Reserve maxBuffer bytes should succeed (exactly at limit, used <= maxBuffer)
	if !ctx.Reserve(int(maxBuf)) {
		t.Fatalf("expected Reserve(%d) to succeed (at limit)", maxBuf)
	}

	// Next reserve should fail (would exceed maxBuffer)
	if ctx.Reserve(1) {
		t.Fatalf("expected Reserve to fail when exceeding maxBuffer")
	}
}

func TestConnContext_Release_DecreasesBufferUsage(t *testing.T) {
	ctx := NewConnContext()

	ctx.Reserve(100)
	ctx.Release(50)

	// After release, should be able to reserve the freed space
	if !ctx.Reserve(50) {
		t.Fatalf("expected Reserve after Release to succeed")
	}
}

func TestConnContext_Allow_InitialTokens(t *testing.T) {
	ctx := NewConnContext()

	// Should have initial tokens (100)
	for i := 0; i < 100; i++ {
		if !ctx.Allow() {
			t.Fatalf("expected Allow() to succeed for initial token %d", i)
		}
	}

	// Next call should fail (no more tokens)
	if ctx.Allow() {
		t.Fatalf("expected Allow() to fail after exhausting tokens")
	}
}

func TestConnContext_Allow_TokenRefillAfterDelay(t *testing.T) {
	ctx := NewConnContext()

	// Exhaust initial tokens
	for i := 0; i < 100; i++ {
		ctx.Allow()
	}

	// Should fail immediately
	if ctx.Allow() {
		t.Fatalf("expected Allow() to fail when out of tokens")
	}

	// Wait 10ms (rate=100 tokens/sec, so 1 token per 10ms)
	time.Sleep(11 * time.Millisecond)

	// Now should have 1+ token
	if !ctx.Allow() {
		t.Fatalf("expected Allow() to succeed after token refill")
	}
}

func TestConnContext_Allow_BurstLimit(t *testing.T) {
	ctx := NewConnContext()

	// Initial tokens: 100, burst: 200
	// Exhaust and wait for refill to hit burst limit
	for i := 0; i < 100; i++ {
		ctx.Allow()
	}

	// Wait 2+ seconds to refill up to burst (200 tokens)
	time.Sleep(2*time.Second + 10*time.Millisecond)

	// Should have exactly burst (200) tokens, not more
	count := 0
	for ctx.Allow() && count < 300 {
		count++
	}

	if count != 200 {
		t.Fatalf("expected exactly 200 tokens (burst limit), got %d", count)
	}
}

func TestConnContext_Allow_ZeroTokensStaysZero(t *testing.T) {
	ctx := NewConnContext()

	// Exhaust all tokens
	for i := 0; i < 100; i++ {
		ctx.Allow()
	}

	// Multiple Allow() calls should all fail
	for i := 0; i < 10; i++ {
		if ctx.Allow() {
			t.Fatalf("expected Allow() to fail consistently when out of tokens (iteration %d)", i)
		}
	}
}
