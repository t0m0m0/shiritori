package srv

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RoomSettings holds configuration for a game room.
type RoomSettings struct {
	Name      string `json:"name"`
	MinLen    int    `json:"minLen"`
	MaxLen    int    `json:"maxLen"`
	Genre     string `json:"genre"`
	TimeLimit int    `json:"timeLimit"`
}

// WordEntry records a word played in the game.
type WordEntry struct {
	Word   string `json:"word"`
	Player string `json:"player"`
	Time   string `json:"time"`
}

// Player represents a connected player.
type Player struct {
	Name string
	Score int
	Conn *websocket.Conn
	Send chan []byte
}

// Room holds the state for a single game room.
type Room struct {
	mu          sync.Mutex
	ID          string        `json:"id"`
	Settings    RoomSettings  `json:"settings"`
	Players     map[string]*Player
	History     []WordEntry   `json:"history"`
	CurrentWord string        `json:"currentWord"`
	Status      string        `json:"status"` // "waiting", "playing", "finished"
	UsedWords   map[string]bool

	timerCancel chan struct{}
	timerLeft   int
}

// RoomManager manages all active rooms.
type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

// NewRoomManager creates a new RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room),
	}
}

// CreateRoom creates a new room with the given settings.
func (rm *RoomManager) CreateRoom(id string, settings RoomSettings) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	room := &Room{
		ID:        id,
		Settings:  settings,
		Players:   make(map[string]*Player),
		History:   []WordEntry{},
		Status:    "waiting",
		UsedWords: make(map[string]bool),
	}
	rm.rooms[id] = room
	return room
}

// GetRoom returns a room by ID.
func (rm *RoomManager) GetRoom(id string) *Room {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.rooms[id]
}

// RemoveRoom removes a room by ID.
func (rm *RoomManager) RemoveRoom(id string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.rooms, id)
}

// ListRooms returns a snapshot of all active rooms.
func (rm *RoomManager) ListRooms() []RoomInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var list []RoomInfo
	for _, r := range rm.rooms {
		r.mu.Lock()
		info := RoomInfo{
			ID:          r.ID,
			Name:        r.Settings.Name,
			PlayerCount: len(r.Players),
			Status:      r.Status,
			Genre:       r.Settings.Genre,
			TimeLimit:   r.Settings.TimeLimit,
		}
		r.mu.Unlock()
		list = append(list, info)
	}
	return list
}

// RoomInfo is a summary of a room for listing.
type RoomInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PlayerCount int    `json:"playerCount"`
	Status      string `json:"status"`
	Genre       string `json:"genre"`
	TimeLimit   int    `json:"timeLimit"`
}

// AddPlayer adds a player to the room.
func (r *Room) AddPlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Players[p.Name] = p
}

// RemovePlayer removes a player from the room and returns the remaining count.
func (r *Room) RemovePlayer(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.Players[name]; ok {
		close(p.Send)
		delete(r.Players, name)
	}
	return len(r.Players)
}

// Broadcast sends a message to all players in the room.
func (r *Room) Broadcast(msg []byte) {
	// Caller should NOT hold r.mu — we lock it here.
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.Players {
		select {
		case p.Send <- msg:
		default:
			// drop if channel full
		}
	}
}

// broadcastLocked sends a message to all players; caller MUST already hold r.mu.
func (r *Room) broadcastLocked(msg []byte) {
	for _, p := range r.Players {
		select {
		case p.Send <- msg:
		default:
		}
	}
}

// StartGame begins the game by picking a random starter word.
func (r *Room) StartGame() (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status == "playing" {
		return "", fmt.Errorf("game already in progress")
	}
	if len(r.Players) < 1 {
		return "", fmt.Errorf("need at least 1 player")
	}

	// Pick random starter word
	word := starterWords[rand.IntN(len(starterWords))]

	r.Status = "playing"
	r.CurrentWord = word
	r.History = []WordEntry{{
		Word:   word,
		Player: "システム",
		Time:   time.Now().Format(time.RFC3339),
	}}
	r.UsedWords = map[string]bool{
		toHiragana(word): true,
	}

	// Start timer if applicable
	if r.Settings.TimeLimit > 0 {
		r.timerLeft = r.Settings.TimeLimit
		r.timerCancel = make(chan struct{})
		go r.runTimer()
	}

	return word, nil
}

// runTimer runs the countdown timer in a goroutine.
func (r *Room) runTimer() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.timerCancel:
			return
		case <-ticker.C:
			r.mu.Lock()
			if r.Status != "playing" {
				r.mu.Unlock()
				return
			}
			r.timerLeft--
			left := r.timerLeft
			if left <= 0 {
				r.Status = "finished"
				msg := mustMarshal(map[string]any{
					"type":    "game_over",
					"reason":  "time_up",
					"scores":  r.getScoresLocked(),
					"history": r.History,
				})
				r.broadcastLocked(msg)
				r.mu.Unlock()
				return
			}
			msg := mustMarshal(map[string]any{
				"type":      "timer",
				"timeLeft":  left,
			})
			r.broadcastLocked(msg)
			r.mu.Unlock()
		}
	}
}

// resetTimer resets the countdown to the room's time limit.
func (r *Room) resetTimer() {
	if r.Settings.TimeLimit > 0 {
		r.timerLeft = r.Settings.TimeLimit
	}
}

// ValidateAndSubmitWord checks a word and applies it if valid.
// Returns (success, message).
func (r *Room) ValidateAndSubmitWord(word, playerName string) (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != "playing" {
		return false, "ゲームが開始されていません"
	}

	// Check that word is valid Japanese kana
	if !isJapanese(word) {
		return false, "ひらがな・カタカナで入力してください"
	}

	hiragana := toHiragana(word)

	// Check length
	wlen := charCount(hiragana)
	if r.Settings.MinLen > 0 && wlen < r.Settings.MinLen {
		return false, fmt.Sprintf("%d文字以上で入力してください", r.Settings.MinLen)
	}
	if r.Settings.MaxLen > 0 && wlen > r.Settings.MaxLen {
		return false, fmt.Sprintf("%d文字以下で入力してください", r.Settings.MaxLen)
	}

	// Check ends with ん
	runes := []rune(hiragana)
	if runes[len(runes)-1] == 'ん' {
		return false, "「ん」で終わる言葉は使えません"
	}

	// Check first char matches last char of current word
	prevHiragana := toHiragana(r.CurrentWord)
	lastChar := getLastChar(prevHiragana)
	firstChar := getFirstChar(hiragana)

	if lastChar != firstChar {
		return false, fmt.Sprintf("「%c」から始まる言葉を入力してください", lastChar)
	}

	// Check not already used
	if r.UsedWords[hiragana] {
		return false, "この言葉はすでに使われています"
	}

	// Genre check
	if !isWordInGenre(hiragana, r.Settings.Genre) {
		return false, fmt.Sprintf("ジャンル「%s」の言葉を入力してください", r.Settings.Genre)
	}

	// All good — apply the word
	r.UsedWords[hiragana] = true
	r.CurrentWord = word
	r.History = append(r.History, WordEntry{
		Word:   word,
		Player: playerName,
		Time:   time.Now().Format(time.RFC3339),
	})

	// Award point
	if p, ok := r.Players[playerName]; ok {
		p.Score++
	}

	// Reset timer
	r.resetTimer()

	return true, ""
}

// getScoresLocked returns a map of player scores. Caller must hold r.mu.
func (r *Room) getScoresLocked() map[string]int {
	scores := make(map[string]int, len(r.Players))
	for name, p := range r.Players {
		scores[name] = p.Score
	}
	return scores
}

// GetScores returns a map of player scores.
func (r *Room) GetScores() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getScoresLocked()
}

// StopTimer cancels the room's timer.
func (r *Room) StopTimer() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timerCancel != nil {
		select {
		case <-r.timerCancel:
			// already closed
		default:
			close(r.timerCancel)
		}
	}
}

// GetState returns a snapshot of the room state for sending to clients.
func (r *Room) GetState() map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()

	players := make([]map[string]any, 0, len(r.Players))
	for name, p := range r.Players {
		players = append(players, map[string]any{
			"name":  name,
			"score": p.Score,
		})
	}

	state := map[string]any{
		"type":        "room_state",
		"roomId":      r.ID,
		"settings":    r.Settings,
		"players":     players,
		"history":     r.History,
		"currentWord": r.CurrentWord,
		"status":      r.Status,
	}
	if r.CurrentWord != "" {
		hiragana := toHiragana(r.CurrentWord)
		state["nextChar"] = string(getLastChar(hiragana))
	}
	if r.Settings.TimeLimit > 0 {
		state["timeLeft"] = r.timerLeft
	}
	return state
}
