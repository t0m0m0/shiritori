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
