package srv

import (
	"testing"
)

func TestChallengeBlocked2Players(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{
			MinLen: 1,
		},
		Players: map[string]*Player{
			"alice": {Name: "alice", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"bob":   {Name: "bob", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		CurrentWord: "",
		Status:      "playing",
		UsedWords:   map[string]bool{},
		History:     []WordEntry{},
		TurnOrder:   []string{"alice", "bob"},
		TurnIndex:   0, // alice's turn
	}

	// Alice submits a word
	result, msg := room.ValidateAndSubmitWord("しりとり", "alice")
	if result != ValidateOK {
		t.Fatalf("expected ValidateOK, got %d: %s", result, msg)
	}

	// Now it's bob's turn. Bob wants to challenge alice's word.
	_, err := room.StartChallengeVote("bob")
	if err != nil {
		t.Errorf("Bob should be able to challenge alice's word, but got error: %s", err)
	}
}

func TestChallengeSelfWordBlocked(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{
			MinLen: 1,
		},
		Players: map[string]*Player{
			"alice": {Name: "alice", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"bob":   {Name: "bob", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		CurrentWord: "",
		Status:      "playing",
		UsedWords:   map[string]bool{},
		History:     []WordEntry{},
		TurnOrder:   []string{"alice", "bob"},
		TurnIndex:   0, // alice's turn
	}

	// Alice submits a word
	result, msg := room.ValidateAndSubmitWord("しりとり", "alice")
	if result != ValidateOK {
		t.Fatalf("expected ValidateOK, got %d: %s", result, msg)
	}

	// Alice tries to challenge her own word — should be blocked
	_, err := room.StartChallengeVote("alice")
	if err == nil {
		t.Error("Alice should NOT be able to challenge her own word")
	}
}

func TestChallenge3Players(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{
			MinLen: 1,
		},
		Players: map[string]*Player{
			"alice":   {Name: "alice", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"bob":     {Name: "bob", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"charlie": {Name: "charlie", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		CurrentWord: "",
		Status:      "playing",
		UsedWords:   map[string]bool{},
		History:     []WordEntry{},
		TurnOrder:   []string{"alice", "bob", "charlie"},
		TurnIndex:   0, // alice's turn
	}

	// Alice submits a word
	result, msg := room.ValidateAndSubmitWord("しりとり", "alice")
	if result != ValidateOK {
		t.Fatalf("expected ValidateOK, got %d: %s", result, msg)
	}

	// Now it's bob's turn. Bob can challenge (even though it's his turn)
	_, err := room.StartChallengeVote("bob")
	if err != nil {
		t.Errorf("Bob should be able to challenge alice's word, but got error: %s", err)
	}

	// Charlie should not be able to also challenge while vote is pending
	_, err = room.StartChallengeVote("charlie")
	if err == nil {
		t.Error("Charlie should NOT be able to start another challenge during a vote")
	}
}

func TestChallengeRejectedRevertsScore(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{
			MinLen: 1,
		},
		Players: map[string]*Player{
			"alice":   {Name: "alice", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"bob":     {Name: "bob", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"charlie": {Name: "charlie", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		CurrentWord: "",
		Status:      "playing",
		UsedWords:   map[string]bool{},
		History:     []WordEntry{},
		TurnOrder:   []string{"alice", "bob", "charlie"},
		TurnIndex:   0, // alice's turn
	}

	// Alice submits a word
	result, msg := room.ValidateAndSubmitWord("しりとり", "alice")
	if result != ValidateOK {
		t.Fatalf("expected ValidateOK, got %d: %s", result, msg)
	}
	if room.Players["alice"].Score != 1 {
		t.Fatalf("expected alice score=1 after word accepted, got %d", room.Players["alice"].Score)
	}

	// Bob challenges alice's word
	_, err := room.StartChallengeVote("bob")
	if err != nil {
		t.Fatalf("failed to start challenge: %s", err)
	}

	// Bob votes reject (auto), charlie votes reject => majority rejects
	room.CastVote("charlie", false)

	// Force resolve if not already resolved
	resolved, voteResult := room.ForceResolveVote()
	if !resolved {
		// May have already been resolved by CastVote
		// Check score directly
	}
	_ = voteResult

	// Alice's score should be reverted back to 0
	if room.Players["alice"].Score != 0 {
		t.Errorf("expected alice score=0 after challenge rejected her word, got %d", room.Players["alice"].Score)
	}

	// Alice should also have lost a life
	if room.Players["alice"].Lives != 2 {
		t.Errorf("expected alice lives=2 after penalty, got %d", room.Players["alice"].Lives)
	}
}

func TestChallengeAcceptedKeepsScore(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{
			MinLen: 1,
		},
		Players: map[string]*Player{
			"alice":   {Name: "alice", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"bob":     {Name: "bob", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"charlie": {Name: "charlie", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
			"dave":    {Name: "dave", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		CurrentWord: "",
		Status:      "playing",
		UsedWords:   map[string]bool{},
		History:     []WordEntry{},
		TurnOrder:   []string{"alice", "bob", "charlie", "dave"},
		TurnIndex:   0, // alice's turn
	}

	// Alice submits a word
	result, msg := room.ValidateAndSubmitWord("しりとり", "alice")
	if result != ValidateOK {
		t.Fatalf("expected ValidateOK, got %d: %s", result, msg)
	}

	// Bob challenges alice's word (auto-votes reject)
	_, err := room.StartChallengeVote("bob")
	if err != nil {
		t.Fatalf("failed to start challenge: %s", err)
	}

	// Charlie and dave vote accept => majority accepts (2 accept vs 1 reject)
	room.CastVote("charlie", true)
	room.CastVote("dave", true)

	// Alice's score should remain 1
	if room.Players["alice"].Score != 1 {
		t.Errorf("expected alice score=1 after challenge accepted her word, got %d", room.Players["alice"].Score)
	}

	// Alice should not have lost a life
	if room.Players["alice"].Lives != 3 {
		t.Errorf("expected alice lives=3 (no penalty), got %d", room.Players["alice"].Lives)
	}
}
