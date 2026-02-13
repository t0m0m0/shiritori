package srv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerSetup(t *testing.T) {
	tempDB := filepath.Join(t.TempDir(), "test_server.sqlite3")
	t.Cleanup(func() { os.Remove(tempDB) })

	server, err := New(tempDB, "test-hostname")
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	if server.Rooms == nil {
		t.Fatal("expected Rooms to be initialized")
	}
}

func TestKanaConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"リンゴ", "りんご"},
		{"しりとり", "しりとり"},
		{"サクラ", "さくら"},
	}
	for _, tt := range tests {
		result := toHiragana(tt.input)
		if result != tt.expected {
			t.Errorf("toHiragana(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetLastChar(t *testing.T) {
	tests := []struct {
		input    string
		expected rune
	}{
		{"しりとり", 'り'},
		{"りんご", 'ご'},
		{"きゃべつ", 'つ'},
		{"ちゃ", 'や'},         // small ya -> ya
		{"らーめん", 'ん'}, // should just be ん
	}
	for _, tt := range tests {
		result := getLastChar(tt.input)
		if result != tt.expected {
			t.Errorf("getLastChar(%q) = %q, want %q", tt.input, string(result), string(tt.expected))
		}
	}
}

func TestGetFirstChar(t *testing.T) {
	tests := []struct {
		input    string
		expected rune
	}{
		{"しりとり", 'し'},
		{"りんご", 'り'},
	}
	for _, tt := range tests {
		result := getFirstChar(tt.input)
		if result != tt.expected {
			t.Errorf("getFirstChar(%q) = %q, want %q", tt.input, string(result), string(tt.expected))
		}
	}
}

func TestIsJapanese(t *testing.T) {
	if !isJapanese("しりとり") {
		t.Error("expected しりとり to be Japanese")
	}
	if !isJapanese("リンゴ") {
		t.Error("expected リンゴ to be Japanese")
	}
	if isJapanese("hello") {
		t.Error("expected hello to not be Japanese")
	}
	if isJapanese("") {
		t.Error("expected empty string to not be Japanese")
	}
}

func TestWordValidation(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{
			MinLen: 2,
			MaxLen: 0,
			Genre:  "",
		},
		Players: map[string]*Player{
			"test": {Name: "test", Score: 0, Send: make(chan []byte, 256)},
		},
		CurrentWord: "しりとり",
		Status:      "playing",
		UsedWords:   map[string]bool{"しりとり": true},
		History:     []WordEntry{},
	}

	// Valid word starting with り
	result, _ := room.ValidateAndSubmitWord("りんご", "test")
	if result != ValidateOK {
		t.Error("expected りんご to be valid")
	}

	// Word ending in ん
	room.CurrentWord = "りんご"
	result, msg := room.ValidateAndSubmitWord("ごはん", "test")
	if result == ValidateOK {
		t.Error("expected ごはん to be rejected (ends in ん)")
	}
	_ = msg

	// Wrong starting character
	result, _ = room.ValidateAndSubmitWord("さくら", "test")
	if result == ValidateOK {
		t.Error("expected さくら to be rejected (doesn't start with ご)")
	}

	// Already used
	room.CurrentWord = "りんご" // last char is ご
	room.UsedWords["ごま"] = true
	result, _ = room.ValidateAndSubmitWord("ごま", "test")
	if result == ValidateOK {
		t.Error("expected ごま to be rejected (already used)")
	}
}

func TestRoomManager(t *testing.T) {
	rm := NewRoomManager()

	// Create room
	room := rm.CreateRoom("test1", RoomSettings{Name: "Test Room"})
	if room == nil {
		t.Fatal("expected room to be created")
	}

	// Get room
	got := rm.GetRoom("test1")
	if got == nil {
		t.Fatal("expected to find room")
	}
	if got.ID != "test1" {
		t.Errorf("expected room ID test1, got %s", got.ID)
	}

	// List rooms
	list := rm.ListRooms()
	if len(list) != 1 {
		t.Errorf("expected 1 room, got %d", len(list))
	}

	// Remove room
	rm.RemoveRoom("test1")
	if rm.GetRoom("test1") != nil {
		t.Error("expected room to be removed")
	}
}

func TestGenreValidation(t *testing.T) {
	// No genre - anything goes
	if !isWordInGenre("あいうえお", "") {
		t.Error("expected any word to pass with no genre")
	}

	// Food genre with known word
	if !isWordInGenre("りんご", "食べ物") {
		t.Error("expected りんご to be in food genre")
	}

	// Food genre with non-food word
	if isWordInGenre("いぬ", "食べ物") {
		t.Error("expected いぬ to not be in food genre")
	}
}
