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

	timerCancel chan struct{}
	timerLeft   int

	// Vote management
	pendingVote *PendingVote
}

// PendingVote holds state for an in-progress genre vote.
type PendingVote struct {
	Word       string
	Hiragana   string
	Player     string
	Challenger string
	Votes      map[string]bool // player name -> accept (true) / reject (false)
	Type       string          // "genre" or "challenge"
	Reason     string
	Timer      *time.Timer
	Resolved   bool
}

// VoteResolution is the outcome of a vote.
type VoteResolution struct {
	Type       string
	Word       string
	Player     string
	Challenger string
	Accepted   bool
	Reverted   bool
}

// VoteInfo describes a new vote request.
type VoteInfo struct {
	Type       string
	Word       string
	Player     string
	Challenger string
	Reason     string
	VoteCount  int
	Total      int
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
func (r *Room) AddPlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Players[p.Name] = p
	r.TurnOrder = append(r.TurnOrder, p.Name)
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

	if r.timerCancel != nil {
		select {
		case <-r.timerCancel:
		default:
			close(r.timerCancel)
		}
		r.timerCancel = nil
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
	r.pendingVote = nil

	// Start timer if applicable
	if r.Settings.TimeLimit > 0 {
		r.timerLeft = r.Settings.TimeLimit
		r.timerCancel = make(chan struct{})
		go r.runTimer()
	}

	return nil
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
				loser := ""
				if len(r.TurnOrder) > 0 && r.TurnIndex < len(r.TurnOrder) {
					loser = r.TurnOrder[r.TurnIndex]
				}
				msg := mustMarshal(map[string]any{
					"type":    "game_over",
					"reason":  "タイムアップ",
					"loser":   loser,
					"scores":  r.getScoresLocked(),
					"history": r.History,
				})
				r.broadcastLocked(msg)
				r.timerCancel = nil
				r.mu.Unlock()
				return
			}
			msg := mustMarshal(map[string]any{
				"type":     "timer",
				"timeLeft": left,
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
	if r.pendingVote != nil && !r.pendingVote.Resolved {
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

	// Genre check — if fails, start a vote (only in multiplayer)
	if !isWordInGenre(hiragana, r.Settings.Genre) {
		// Solo play: no vote possible, just reject
		if len(r.Players) <= 1 {
			return ValidateRejected, fmt.Sprintf("ジャンル「%s」の言葉を入力してください", r.Settings.Genre)
		}
		// Start a vote
		r.pendingVote = &PendingVote{
			Word:     word,
			Hiragana: hiragana,
			Player:   playerName,
			Votes:    make(map[string]bool),
			Type:     "genre",
			Reason:   fmt.Sprintf("「%s」はジャンル「%s」のリストにありません", word, r.Settings.Genre),
		}
		// Submitter's vote automatically counts as accept
		r.pendingVote.Votes[playerName] = true
		return ValidateVote, fmt.Sprintf("「%s」はジャンル「%s」のリストにありません。投票で判定します", word, r.Settings.Genre)
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
	r.pendingVote = nil
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

// CastVote records a player's vote and returns resolution if complete.
func (r *Room) CastVote(playerName string, accept bool) (resolved bool, result VoteResolution) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pendingVote == nil || r.pendingVote.Resolved {
		return false, VoteResolution{}
	}

	// Only players in the room can vote
	if _, ok := r.Players[playerName]; !ok {
		return false, VoteResolution{}
	}

	r.pendingVote.Votes[playerName] = accept

	// Check if all players have voted
	if len(r.pendingVote.Votes) < len(r.Players) {
		return false, VoteResolution{}
	}

	return r.resolveVoteLocked()
}

// ForceResolveVote resolves the vote by timeout (majority wins, tie = reject).
func (r *Room) ForceResolveVote() (resolved bool, result VoteResolution) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pendingVote == nil || r.pendingVote.Resolved {
		return false, VoteResolution{}
	}

	return r.resolveVoteLocked()
}

func (r *Room) resolveVoteLocked() (resolved bool, result VoteResolution) {
	r.pendingVote.Resolved = true
	acceptCount := 0
	rejectCount := 0
	for _, v := range r.pendingVote.Votes {
		if v {
			acceptCount++
		} else {
			rejectCount++
		}
	}
	// Non-voters count as reject
	rejectCount += len(r.Players) - len(r.pendingVote.Votes)

	accepted := acceptCount > rejectCount
	result = VoteResolution{
		Type:       r.pendingVote.Type,
		Word:       r.pendingVote.Word,
		Player:     r.pendingVote.Player,
		Challenger: r.pendingVote.Challenger,
		Accepted:   accepted,
	}

	if r.pendingVote.Type == "genre" {
		if accepted {
			r.applyWordLocked(r.pendingVote.Word, r.pendingVote.Hiragana, r.pendingVote.Player)
		} else {
			// Vote rejected — clear pending vote, player keeps their turn
			r.pendingVote = nil
		}
		return true, result
	}

	// Challenge vote: accepted = word stays; rejected = word removed, challenger plays.
	if accepted {
		r.pendingVote = nil
		return true, result
	}

	// Revert last word and hand turn to challenger.
	if len(r.History) > 0 {
		r.History = r.History[:len(r.History)-1]
	}
	delete(r.UsedWords, r.pendingVote.Hiragana)
	prevWord := ""
	if len(r.History) > 0 {
		prevWord = r.History[len(r.History)-1].Word
	}
	challengerIndex := -1
	for i, name := range r.TurnOrder {
		if name == r.pendingVote.Challenger {
			challengerIndex = i
			break
		}
	}
	if challengerIndex >= 0 {
		r.TurnIndex = challengerIndex
	}
	result.Reverted = true
	result.Player = r.pendingVote.Player
	result.Challenger = r.pendingVote.Challenger
	result.Word = r.pendingVote.Word
	result.Accepted = false

	// Restore current word to previous
	r.CurrentWord = prevWord
	// Reset timer
	r.resetTimer()

	r.pendingVote = nil
	return true, result
}

// StartChallengeVote starts a vote to challenge a word.
func (r *Room) StartChallengeVote(challengerName string) (VoteInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != "playing" {
		return VoteInfo{}, fmt.Errorf("ゲームが開始されていません")
	}
	if r.pendingVote != nil && !r.pendingVote.Resolved {
		return VoteInfo{}, fmt.Errorf("投票中です。投票が終わるまでお待ちください")
	}
	if len(r.History) == 0 {
		return VoteInfo{}, fmt.Errorf("まだ単語がありません")
	}
	if _, ok := r.Players[challengerName]; !ok {
		return VoteInfo{}, fmt.Errorf("ルームに参加していません")
	}
	last := r.History[len(r.History)-1]
	if last.Player == challengerName {
		return VoteInfo{}, fmt.Errorf("自分の単語には指摘できません")
	}
	if r.TurnOrder[r.TurnIndex] == challengerName {
		return VoteInfo{}, fmt.Errorf("自分の番では指摘できません")
	}
	hiragana := toHiragana(last.Word)

	r.pendingVote = &PendingVote{
		Word:       last.Word,
		Hiragana:   hiragana,
		Player:     last.Player,
		Challenger: challengerName,
		Votes:      make(map[string]bool),
		Type:       "challenge",
		Reason:     fmt.Sprintf("「%s」は存在しない単語かもしれません", last.Word),
	}

	// Challenger auto-votes reject (word should be removed)
	r.pendingVote.Votes[challengerName] = false

	info := VoteInfo{
		Type:       "challenge",
		Word:       last.Word,
		Player:     last.Player,
		Challenger: challengerName,
		Reason:     r.pendingVote.Reason,
		VoteCount:  len(r.pendingVote.Votes),
		Total:      len(r.Players),
	}
	return info, nil
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
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timerCancel != nil {
		select {
		case <-r.timerCancel:
			// already closed
		default:
			close(r.timerCancel)
		}
		r.timerCancel = nil
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

	if r.Settings.TimeLimit > 0 {
		state["timeLeft"] = r.timerLeft
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
