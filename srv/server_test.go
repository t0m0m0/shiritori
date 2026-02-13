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
		{"るびー", 'び'},     // long vowel: use preceding char
		{"こーひー", 'ひ'},   // long vowel: use preceding char
		{"ぎたー", 'た'},     // long vowel: use preceding char
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
	settings := RoomSettings{
		MinLen: 2,
		MaxLen: 0,
		Genre:  "",
	}
	room := &Room{
		Settings: settings,
		Players: map[string]*Player{
			"test": {Name: "test", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		Status: "playing",
	}
	room.Engine = NewGameEngine(settings, []string{"test"}, nil)
	// Pre-set game state
	room.Engine.CurrentWord = "しりとり"
	room.Engine.UsedWords["しりとり"] = true

	// Valid word starting with り
	result, _ := room.ValidateAndSubmitWord("りんご", "test")
	if result != ValidateOK {
		t.Error("expected りんご to be valid")
	}

	// Word ending in ん — now a penalty (word accepted but player loses a life)
	room.Engine.CurrentWord = "りんご"
	result, msg := room.ValidateAndSubmitWord("ごはん", "test")
	if result != ValidatePenalty {
		t.Errorf("expected ごはん to be ValidatePenalty, got result=%d msg=%s", result, msg)
	}
	_ = msg

	// Penalty does not change current word, so still need to start with ご
	result, _ = room.ValidateAndSubmitWord("さくら", "test")
	if result == ValidateOK {
		t.Error("expected さくら to be rejected (doesn't start with ご)")
	}

	// Already used
	room.Engine.CurrentWord = "りんご" // last char is ご
	room.Engine.UsedWords["ごま"] = true
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

func TestGetKanaRow(t *testing.T) {
	tests := []struct {
		char     rune
		expected string
	}{
		{'あ', "あ行"},
		{'い', "あ行"},
		{'か', "か行"},
		{'が', "か行"},
		{'さ', "さ行"},
		{'じ', "さ行"},
		{'た', "た行"},
		{'ば', "は行"},
		{'ぱ', "は行"},
		{'ん', "わ行"},
	}
	for _, tt := range tests {
		result := GetKanaRow(tt.char)
		if result != tt.expected {
			t.Errorf("GetKanaRow(%q) = %q, want %q", string(tt.char), result, tt.expected)
		}
	}
}

func TestValidateAllowedRows(t *testing.T) {
	// Only あ行 and か行 allowed
	allowed := []string{"あ行", "か行"}

	// "あか" uses あ行 and か行 - should pass
	badChar, _ := ValidateAllowedRows("あか", allowed)
	if badChar != 0 {
		t.Error("expected あか to pass with あ行+か行")
	}

	// "さかな" uses さ行 - should fail
	badChar, badRow := ValidateAllowedRows("さかな", allowed)
	if badChar == 0 {
		t.Error("expected さかな to fail with あ行+か行")
	}
	if badRow != "さ行" {
		t.Errorf("expected badRow=さ行, got %q", badRow)
	}

	// Empty allowed = all pass
	badChar, _ = ValidateAllowedRows("さかな", nil)
	if badChar != 0 {
		t.Error("expected さかな to pass with no restriction")
	}

	// Dakuten: が should be in か行
	badChar, _ = ValidateAllowedRows("がき", allowed)
	if badChar != 0 {
		t.Error("expected がき to pass with か行 allowed")
	}
}

func TestAllowedRowsInGame(t *testing.T) {
	settings := RoomSettings{
		MinLen:      1,
		AllowedRows: []string{"あ行", "か行"},
	}
	room := &Room{
		Settings: settings,
		Players: map[string]*Player{
			"test": {Name: "test", Score: 0, Lives: 3, Send: make(chan []byte, 256)},
		},
		Status: "playing",
	}
	room.Engine = NewGameEngine(settings, []string{"test"}, nil)

	// "あき" - all chars in あ行/か行 - should pass
	result, _ := room.ValidateAndSubmitWord("あき", "test")
	if result != ValidateOK {
		t.Error("expected あき to be valid with あ行+か行")
	}

	// "きた" - た is in た行 - should be a penalty (word accepted, life lost)
	room.Engine.CurrentWord = "あき"
	result, msg := room.ValidateAndSubmitWord("きた", "test")
	if result != ValidatePenalty {
		t.Errorf("expected きた to be ValidatePenalty, got result=%d msg=%s", result, msg)
	}
}
