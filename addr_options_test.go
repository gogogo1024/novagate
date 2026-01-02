package novagate

import "testing"

func TestNormalizeAddrRules(t *testing.T) {
	// Backward-compatible: explicit addr always wins.
	if got := normalizeAddr("127.0.0.1:1234", []ServeOption{WithAddr(":9000")}); got != "127.0.0.1:1234" {
		t.Fatalf("normalizeAddr explicit addr: got %q", got)
	}

	// Default when nothing provided.
	if got := normalizeAddr("", nil); got != ":9000" {
		t.Fatalf("normalizeAddr default: got %q", got)
	}

	// Option-based addr when explicit addr is empty.
	if got := normalizeAddr("", []ServeOption{WithAddr("0.0.0.0:7777")}); got != "0.0.0.0:7777" {
		t.Fatalf("normalizeAddr WithAddr: got %q", got)
	}
}
