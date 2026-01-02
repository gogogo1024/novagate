package novagate

import (
	"testing"
	"time"
)

func TestNextAcceptBackoffDoublesUntilCapped(t *testing.T) {
	cur := 5 * time.Millisecond
	cur = nextAcceptBackoff(cur)
	if cur != 10*time.Millisecond {
		t.Fatalf("expected 10ms, got %s", cur)
	}
	cur = nextAcceptBackoff(cur)
	if cur != 20*time.Millisecond {
		t.Fatalf("expected 20ms, got %s", cur)
	}
}

func TestNextAcceptBackoffCappedAtOneSecond(t *testing.T) {
	cur := 800 * time.Millisecond
	cur = nextAcceptBackoff(cur)
	if cur != time.Second {
		t.Fatalf("expected 1s cap, got %s", cur)
	}
	cur = nextAcceptBackoff(cur)
	if cur != time.Second {
		t.Fatalf("expected to stay capped at 1s, got %s", cur)
	}
}
