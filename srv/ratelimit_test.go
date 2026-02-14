package srv

import (
	"testing"
	"time"
)

func TestTokenBucket_BasicAllow(t *testing.T) {
	tb := newTokenBucket(10, 3) // 10/sec, burst 3
	// Should allow up to burst
	for i := 0; i < 3; i++ {
		if !tb.allow() {
			t.Fatalf("expected allow on request %d", i)
		}
	}
	// 4th should be denied
	if tb.allow() {
		t.Fatal("expected deny after burst exhausted")
	}
}

func TestTokenBucket_Refill(t *testing.T) {
	tb := newTokenBucket(10, 3)
	// Exhaust
	for i := 0; i < 3; i++ {
		tb.allow()
	}
	// Wait for refill (100ms = 1 token at 10/sec)
	time.Sleep(150 * time.Millisecond)
	if !tb.allow() {
		t.Fatal("expected allow after refill")
	}
}

func TestConnectionRateLimiter_AllowNormal(t *testing.T) {
	rl := NewConnectionRateLimiter()
	// Normal usage should be allowed
	for i := 0; i < 5; i++ {
		allowed, disconnect := rl.Allow("get_rooms")
		if !allowed {
			t.Fatalf("expected allow on request %d", i)
		}
		if disconnect {
			t.Fatal("unexpected disconnect")
		}
	}
}

func TestConnectionRateLimiter_PerTypeLimit(t *testing.T) {
	rl := NewConnectionRateLimiter()
	// answer: burst=3, so 4th should be denied
	for i := 0; i < 3; i++ {
		allowed, _ := rl.Allow("answer")
		if !allowed {
			t.Fatalf("expected allow on answer %d", i)
		}
	}
	allowed, _ := rl.Allow("answer")
	if allowed {
		t.Fatal("expected deny on answer after burst")
	}
}

func TestConnectionRateLimiter_GlobalLimit(t *testing.T) {
	rl := NewConnectionRateLimiter()
	// Global burst is 20; exhaust it with mixed types
	denied := false
	for i := 0; i < 30; i++ {
		allowed, _ := rl.Allow("ping")
		if !allowed {
			denied = true
			break
		}
	}
	if !denied {
		t.Fatal("expected global rate limit to kick in")
	}
}

func TestConnectionRateLimiter_DisconnectOnExcessiveViolations(t *testing.T) {
	rl := NewConnectionRateLimiter()
	// Exhaust per-type burst first
	for i := 0; i < 3; i++ {
		rl.Allow("answer")
	}
	// Now spam to accumulate violations
	disconnected := false
	for i := 0; i < 100; i++ {
		_, shouldDisconnect := rl.Allow("answer")
		if shouldDisconnect {
			disconnected = true
			break
		}
	}
	if !disconnected {
		t.Fatal("expected disconnect after excessive violations")
	}
}

func TestConnectionRateLimiter_UnknownType(t *testing.T) {
	rl := NewConnectionRateLimiter()
	// Unknown types get strict default (burst=2)
	allowed1, _ := rl.Allow("unknown_type")
	allowed2, _ := rl.Allow("unknown_type")
	allowed3, _ := rl.Allow("unknown_type")
	if !allowed1 || !allowed2 {
		t.Fatal("expected first 2 unknown type messages to be allowed")
	}
	if allowed3 {
		t.Fatal("expected 3rd unknown type message to be denied")
	}
}
