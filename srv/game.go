package srv

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RoomSettings holds configuration for a game room.
type RoomSettings struct {
	Name        string   `json:"name"`
	MinLen      int      `json:"minLen"`
	MaxLen      int      `json:"maxLen"`
	Genre       string   `json:"genre"`
	TimeLimit   int      `json:"timeLimit"`
	AllowedRows []string `json:"allowedRows,omitempty"` // e.g. ["あ行","か行"]; empty = all rows allowed
	NoDakuten   bool     `json:"noDakuten,omitempty"`   // disallow dakuten/handakuten characters
	MaxLives    int      `json:"maxLives"`              // max lives per player (default 3 if 0)
	Private     bool     `json:"private,omitempty"`      // if true, room is hidden from lobby list
}

// WordEntry records a word played in the game.
type WordEntry struct {
	Word   string `json:"word"`
	Player string `json:"player"`
	Time   string `json:"time"`
}

// Player represents a connected player.
type Player struct {
	Name  string
	Score int
	Lives int
	Conn  *websocket.Conn
	Send  chan []byte
}

// Room holds the state for a single game room.
type Room struct {
	mu          sync.Mutex
	ID          string       `json:"id"`
	Owner       string       `json:"owner"`
	Settings    RoomSettings `json:"settings"`
	Players     map[string]*Player
	History     []WordEntry `json:"history"`
	CurrentWord string      `json:"currentWord"`
	Status      string      `json:"status"` // "waiting", "playing", "finished"
	UsedWords   map[string]bool

	// Turn management
	TurnOrder []string // player names in order
	TurnIndex int      // index into TurnOrder for current turn

	Timer *TimerManager

	// Callback for saving game result on game over (set by Server)
	OnGameOver func(room *Room, result map[string]any) map[string]any

	// Vote management
	Votes *VoteManager
}



// RoomManager manages all active rooms.
type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	// playerRoom tracks which room each player name is currently in.
	playerRoom map[string]string // player name -> room ID
}

// NewRoomManager creates a new RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:      make(map[string]*Room),
		playerRoom: make(map[string]string),
	}
}

// TrackPlayer records that a player is in a room.
func (rm *RoomManager) TrackPlayer(name, roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.playerRoom[name] = roomID
}

// UntrackPlayer removes a player's room tracking.
func (rm *RoomManager) UntrackPlayer(name string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	delete(rm.playerRoom, name)
}

// PlayerRoomID returns the room ID the player is in, or "" if none.
func (rm *RoomManager) PlayerRoomID(name string) string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.playerRoom[name]
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
		if r.Settings.Private {
			r.mu.Unlock()
			continue
		}
		info := RoomInfo{
			ID:          r.ID,
			Name:        r.Settings.Name,
			PlayerCount: len(r.Players),
			Status:      r.Status,
			Genre:       r.Settings.Genre,
			TimeLimit:   r.Settings.TimeLimit,
			Owner:       r.Owner,
			Settings:    r.Settings,
		}
		r.mu.Unlock()
		list = append(list, info)
	}
	return list
}

// RoomInfo is a summary of a room for listing.
type RoomInfo struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	PlayerCount int          `json:"playerCount"`
	Status      string       `json:"status"`
	Genre       string       `json:"genre"`
	TimeLimit   int          `json:"timeLimit"`
	Owner       string       `json:"owner"`
	Settings    RoomSettings `json:"settings"`
}

// AddPlayer adds a player to the room.
// If the game is already playing, the player is inserted into TurnOrder
// right after the last position so they participate from the next full round,
// and their lives are initialized.
func (r *Room) AddPlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Players[p.Name] = p

	if r.Status == "playing" {
		// Initialize lives for mid-game joiner
		maxLives := r.Settings.MaxLives
		if maxLives <= 0 {
			maxLives = 3
		}
		p.Lives = maxLives
		p.Score = 0

		// Insert right after the current turn index so the new player
		// gets their first turn at the end of the current round.
		// Place them at the end of TurnOrder (they'll play after everyone
		// who was already in the order).
		r.TurnOrder = append(r.TurnOrder, p.Name)
	} else {
		r.TurnOrder = append(r.TurnOrder, p.Name)
	}
}

// PlayerNames returns a snapshot of current player names.
func (r *Room) PlayerNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	names := make([]string, 0, len(r.Players))
	for name := range r.Players {
		names = append(names, name)
	}
	return names
}

// RemovePlayer removes a player from the room and returns the remaining count.
func (r *Room) RemovePlayer(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.Players[name]; ok {
		close(p.Send)
		delete(r.Players, name)
	}
	// Remove from turn order
	for i, n := range r.TurnOrder {
		if n == name {
			r.TurnOrder = append(r.TurnOrder[:i], r.TurnOrder[i+1:]...)
			// Adjust TurnIndex if needed
			if len(r.TurnOrder) > 0 {
				if r.TurnIndex >= len(r.TurnOrder) {
					r.TurnIndex = 0
				}
			}
			break
		}
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

// StartGame begins the game. The room owner goes first and picks any word.
func (r *Room) StartGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status == "playing" {
		return fmt.Errorf("game already in progress")
	}
	if len(r.Players) < 1 {
		return fmt.Errorf("need at least 1 player")
	}

	if r.Timer != nil {
		r.Timer.Stop()
	}

	r.Status = "playing"
	r.CurrentWord = "" // owner picks the first word

	// Build turn order with owner first, rest shuffled
	r.TurnOrder = make([]string, 0, len(r.Players))
	for name := range r.Players {
		if name != r.Owner {
			r.TurnOrder = append(r.TurnOrder, name)
		}
	}
	rand.Shuffle(len(r.TurnOrder), func(i, j int) {
		r.TurnOrder[i], r.TurnOrder[j] = r.TurnOrder[j], r.TurnOrder[i]
	})
	r.TurnOrder = append([]string{r.Owner}, r.TurnOrder...)
	r.TurnIndex = 0

	// Reset scores and initialize lives
	maxLives := r.Settings.MaxLives
	if maxLives <= 0 {
		maxLives = 3
	}
	for _, p := range r.Players {
		p.Score = 0
		p.Lives = maxLives
	}

	r.History = []WordEntry{}
	r.UsedWords = make(map[string]bool)
	if r.Votes != nil {
		r.Votes.Clear()
	}

	// Start timer if applicable
	if r.Settings.TimeLimit > 0 && r.Timer != nil {
		r.Timer.Start(r.Settings.TimeLimit)
	}

	return nil
}

// UpdateSettings updates the room settings. Only allowed when game is not playing.
func (r *Room) UpdateSettings(s RoomSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Status == "playing" {
		return fmt.Errorf("ゲーム中は設定を変更できません")
	}
	// Preserve room name and private flag from original settings if not provided
	if s.Name == "" {
		s.Name = r.Settings.Name
	}
	r.Settings = s
	return nil
}

// resetTimer resets the countdown to the room's time limit.
func (r *Room) resetTimer() {
	if r.Timer != nil {
		r.Timer.Reset()
	}
}

// ValidateResult represents the outcome of word validation.
type ValidateResult int

const (
	ValidateOK       ValidateResult = iota // Word accepted
	ValidateRejected                       // Word rejected (hard fail)
	ValidateVote                           // Need genre vote
	ValidatePenalty                        // Word accepted but player loses a life
)

// ValidateAndSubmitWord checks a word and applies it if valid.
// Returns (result, message). If result is ValidateVote, a vote has been started.
func (r *Room) ValidateAndSubmitWord(word, playerName string) (ValidateResult, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != "playing" {
		return ValidateRejected, "ゲームが開始されていません"
	}

	// Reject if a vote is in progress
	if r.Votes != nil && r.Votes.HasPendingVote() {
		return ValidateRejected, "投票中です。投票が終わるまでお待ちください"
	}

	// Check it's this player's turn
	if len(r.TurnOrder) > 0 && r.TurnOrder[r.TurnIndex] != playerName {
		return ValidateRejected, fmt.Sprintf("%sさんの番です", r.TurnOrder[r.TurnIndex])
	}

	// Check player is not eliminated
	if p, ok := r.Players[playerName]; ok && p.Lives <= 0 {
		return ValidateRejected, "あなたは脱落済みです"
	}

	// Check that word is valid Japanese kana
	if !isJapanese(word) {
		return ValidateRejected, "ひらがな・カタカナで入力してください"
	}

	hiragana := toHiragana(word)

	// Check length
	wlen := charCount(hiragana)
	if r.Settings.MinLen > 0 && wlen < r.Settings.MinLen {
		return ValidateRejected, fmt.Sprintf("%d文字以上で入力してください", r.Settings.MinLen)
	}
	if r.Settings.MaxLen > 0 && wlen > r.Settings.MaxLen {
		return ValidateRejected, fmt.Sprintf("%d文字以下で入力してください", r.Settings.MaxLen)
	}

	// Check first char matches last char of current word (skip for first word)
	if r.CurrentWord != "" {
		prevHiragana := toHiragana(r.CurrentWord)
		lastChar := getLastChar(prevHiragana)
		firstChar := getFirstChar(hiragana)

		if lastChar != firstChar {
			return ValidateRejected, fmt.Sprintf("「%c」から始まる言葉を入力してください", lastChar)
		}
	}

	// Check not already used — penalty (lose a life)
	if r.UsedWords[hiragana] {
		r.applyPenaltyLocked(playerName)
		return ValidatePenalty, "この言葉はすでに使われています"
	}

	// --- Penalty checks: word NOT accepted, but player loses a life ---

	// Check ends with ん
	runes := []rune(hiragana)
	if runes[len(runes)-1] == 'ん' {
		r.applyPenaltyLocked(playerName)
		return ValidatePenalty, "「ん」で終わる言葉を使いました"
	}

	// Check no dakuten/handakuten
	if r.Settings.NoDakuten {
		if badChar := ValidateNoDakuten(hiragana); badChar != 0 {
			r.applyPenaltyLocked(playerName)
			return ValidatePenalty, fmt.Sprintf("「%c」は濁音・半濁音の文字です（濁音・半濁音禁止ルール）", badChar)
		}
	}

	// Check allowed rows
	if len(r.Settings.AllowedRows) > 0 {
		if badChar, badRow := ValidateAllowedRows(hiragana, r.Settings.AllowedRows); badChar != 0 {
			r.applyPenaltyLocked(playerName)
			return ValidatePenalty, fmt.Sprintf("「%c」は%sの文字です（使用可能な行: %s）", badChar, badRow, formatAllowedRows(r.Settings.AllowedRows))
		}
	}

	// All good — apply the word
	r.applyWordLocked(word, hiragana, playerName)
	return ValidateOK, ""
}

// applyWordLocked applies an accepted word. Caller must hold r.mu.
func (r *Room) applyWordLocked(word, hiragana, playerName string) {
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

	// Advance turn, skipping eliminated players
	if len(r.TurnOrder) > 0 {
		start := r.TurnIndex
		for {
			r.TurnIndex = (r.TurnIndex + 1) % len(r.TurnOrder)
			// If we cycled all the way back, stop (avoid infinite loop)
			if r.TurnIndex == start {
				break
			}
			// If the current turn player is alive, stop
			nextName := r.TurnOrder[r.TurnIndex]
			if p, ok := r.Players[nextName]; ok && p.Lives > 0 {
				break
			}
		}
	}

	// Reset timer
	r.resetTimer()

	// Clear any resolved vote
	if r.Votes != nil {
		r.Votes.Clear()
	}
}

// applyPenaltyLocked decrements a player's lives. Caller must hold r.mu.
func (r *Room) applyPenaltyLocked(playerName string) {
	if p, ok := r.Players[playerName]; ok {
		p.Lives--
	}
}

// getAlivePlayers returns the names of players with lives > 0. Caller must hold r.mu.
func (r *Room) getAlivePlayers() []string {
	var alive []string
	for name, p := range r.Players {
		if p.Lives > 0 {
			alive = append(alive, name)
		}
	}
	return alive
}

// checkElimination checks if a player is eliminated and whether the game is over.
// Returns (eliminated, gameOver, lastSurvivor). Caller must hold r.mu.
func (r *Room) checkElimination(playerName string) (eliminated bool, gameOver bool, lastSurvivor string) {
	if p, ok := r.Players[playerName]; ok && p.Lives <= 0 {
		eliminated = true
	}
	alive := r.getAlivePlayers()
	totalPlayers := len(r.Players)
	if totalPlayers <= 1 {
		// Solo play: game over only when player is eliminated
		if len(alive) == 0 {
			gameOver = true
		}
	} else {
		// Multiplayer: game over when 1 or fewer alive
		if len(alive) <= 1 {
			gameOver = true
			if len(alive) == 1 {
				lastSurvivor = alive[0]
			}
		}
	}
	return
}

// getLivesLocked returns a map of player lives. Caller must hold r.mu.
func (r *Room) getLivesLocked() map[string]int {
	lives := make(map[string]int, len(r.Players))
	for name, p := range r.Players {
		lives[name] = p.Lives
	}
	return lives
}

// GetLives returns a map of player lives (public, acquires lock).
func (r *Room) GetLives() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.getLivesLocked()
}

// CastVote delegates to VoteManager. If resolved and challenge rejected, applies revert to game state.
func (r *Room) CastVote(playerName string, accept bool) (resolved bool, result VoteResolution) {
	resolved, result = r.Votes.CastVote(playerName, accept)
	if resolved {
		r.applyVoteResult(&result)
	}
	return
}

// ForceResolveVote delegates to VoteManager. Applies revert if challenge rejected.
func (r *Room) ForceResolveVote() (resolved bool, result VoteResolution) {
	resolved, result = r.Votes.ForceResolveVote()
	if resolved {
		r.applyVoteResult(&result)
	}
	return
}

// applyVoteResult applies the game-state side effects of a resolved vote.
func (r *Room) applyVoteResult(result *VoteResolution) {
	if result.Type == "genre" {
		if result.Accepted {
			// Get the pending vote info before it's cleared
			pv := r.Votes.GetPending()
			if pv != nil {
				r.mu.Lock()
				r.applyWordLocked(pv.Word, pv.Hiragana, pv.Player)
				r.mu.Unlock()
			}
		}
		r.Votes.Clear()
		return
	}

	// Challenge vote
	if result.Accepted || !result.Reverted {
		return
	}

	// Challenge rejected — revert the word
	r.mu.Lock()
	if len(r.History) > 0 {
		r.History = r.History[:len(r.History)-1]
	}
	delete(r.UsedWords, toHiragana(result.Word))

	// Revert score
	if p, ok := r.Players[result.Player]; ok {
		if p.Score > 0 {
			p.Score--
		}
	}

	prevWord := ""
	if len(r.History) > 0 {
		prevWord = r.History[len(r.History)-1].Word
	}

	// Turn stays with the original player
	for i, name := range r.TurnOrder {
		if name == result.Player {
			r.TurnIndex = i
			break
		}
	}

	// Penalize the original player
	r.applyPenaltyLocked(result.Player)

	r.CurrentWord = prevWord
	r.resetTimer()
	r.mu.Unlock()
}

// StartChallengeVote starts a vote to challenge the last word.
func (r *Room) StartChallengeVote(challengerName string) (VoteInfo, error) {
	r.mu.Lock()
	if r.Status != "playing" {
		r.mu.Unlock()
		return VoteInfo{}, fmt.Errorf("ゲームが開始されていません")
	}
	if len(r.History) == 0 {
		r.mu.Unlock()
		return VoteInfo{}, fmt.Errorf("まだ単語がありません")
	}
	last := r.History[len(r.History)-1]
	r.mu.Unlock()

	playerExists := func(name string) bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		_, ok := r.Players[name]
		return ok
	}
	return r.Votes.StartChallengeVote(challengerName, last, playerExists)
}

// WithdrawChallenge delegates to VoteManager.
func (r *Room) WithdrawChallenge(challengerName string) bool {
	return r.Votes.WithdrawChallenge(challengerName)
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

// formatAllowedRows returns a comma-separated list of allowed row names.
func formatAllowedRows(rows []string) string {
	return strings.Join(rows, "・")
}

// StopTimer cancels the room's timer.
func (r *Room) StopTimer() {
	if r.Timer != nil {
		r.Timer.Stop()
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

	if r.Settings.TimeLimit > 0 && r.Timer != nil {
		state["timeLeft"] = r.Timer.TimeLeft()
	}
	state["turnOrder"] = r.TurnOrder
	if len(r.TurnOrder) > 0 && r.TurnIndex < len(r.TurnOrder) {
		state["currentTurn"] = r.TurnOrder[r.TurnIndex]
	}
	state["owner"] = r.Owner
	state["lives"] = r.getLivesLocked()
	maxLives := r.Settings.MaxLives
	if maxLives <= 0 {
		maxLives = 3
	}
	state["maxLives"] = maxLives
	return state
}
