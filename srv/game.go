package srv

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// roomCleanupInterval is how often the cleanup goroutine checks for empty rooms.
	roomCleanupInterval = 1 * time.Minute
	// roomMaxEmptyAge is how long a room can stay empty before being removed.
	roomMaxEmptyAge = 5 * time.Minute
	// defaultMaxPlayers is the default maximum number of players per room.
	defaultMaxPlayers = 8
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
	MaxPlayers  int      `json:"maxPlayers,omitempty"`   // max players per room (default 8 if 0)
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
	mu       sync.Mutex
	ID       string       `json:"id"`
	Owner    string       `json:"owner"`
	Settings RoomSettings `json:"settings"`
	Players  map[string]*Player
	Status   string `json:"status"` // "waiting", "playing", "finished"

	// Composed managers
	Engine *GameEngine
	Timer  *TimerManager
	Votes  *VoteManager

	// Callback for saving game result on game over (set by Server)
	OnGameOver func(room *Room, result map[string]any) map[string]any

	// EmptySince tracks when the room became empty; nil if room has players.
	EmptySince *time.Time
}



// RoomManager manages all active rooms.
type RoomManager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	// playerRoom tracks which room each player name is currently in.
	playerRoom map[string]string // player name -> room ID
	// done is used to stop the cleanup goroutine.
	done chan struct{}
}

// NewRoomManager creates a new RoomManager.
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms:      make(map[string]*Room),
		playerRoom: make(map[string]string),
		done:       make(chan struct{}),
	}
}

// StartCleanup starts a background goroutine that periodically removes
// rooms that have been empty for longer than maxEmptyAge.
func (rm *RoomManager) StartCleanup(interval, maxEmptyAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-rm.done:
				return
			case <-ticker.C:
				rm.cleanupEmptyRooms(maxEmptyAge)
			}
		}
	}()
}

// StopCleanup stops the background cleanup goroutine.
func (rm *RoomManager) StopCleanup() {
	close(rm.done)
}

// cleanupEmptyRooms removes rooms that have been empty longer than maxAge.
func (rm *RoomManager) cleanupEmptyRooms(maxAge time.Duration) {
	now := time.Now()
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for id, r := range rm.rooms {
		r.mu.Lock()
		if r.EmptySince != nil && now.Sub(*r.EmptySince) > maxAge {
			r.mu.Unlock()
			delete(rm.rooms, id)
			slog.Info("room cleaned up (empty timeout)", "roomId", id)
		} else {
			r.mu.Unlock()
		}
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
		ID:       id,
		Settings: settings,
		Players:  make(map[string]*Player),
		Status:   "waiting",
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
		maxP := r.Settings.MaxPlayers
		if maxP <= 0 {
			maxP = defaultMaxPlayers
		}
		info := RoomInfo{
			ID:          r.ID,
			Name:        r.Settings.Name,
			PlayerCount: len(r.Players),
			MaxPlayers:  maxP,
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
	MaxPlayers  int          `json:"maxPlayers"`
	Status      string       `json:"status"`
	Genre       string       `json:"genre"`
	TimeLimit   int          `json:"timeLimit"`
	Owner       string       `json:"owner"`
	Settings    RoomSettings `json:"settings"`
}

// MaxPlayersLimit returns the effective max player limit for this room.
func (r *Room) MaxPlayersLimit() int {
	if r.Settings.MaxPlayers > 0 {
		return r.Settings.MaxPlayers
	}
	return defaultMaxPlayers
}

// AddPlayer adds a player to the room.
func (r *Room) AddPlayer(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Players[p.Name] = p
	r.EmptySince = nil

	if r.Status == "playing" && r.Engine != nil {
		r.Engine.AddPlayer(p.Name)
		// Sync player connection-level state
		if ps, ok := r.Engine.Players[p.Name]; ok {
			p.Lives = ps.Lives
			p.Score = ps.Score
		}
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
	if r.Engine != nil {
		r.Engine.RemovePlayer(name)
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

	// Build turn order with owner first, rest shuffled
	turnOrder := make([]string, 0, len(r.Players))
	for name := range r.Players {
		if name != r.Owner {
			turnOrder = append(turnOrder, name)
		}
	}
	rand.Shuffle(len(turnOrder), func(i, j int) {
		turnOrder[i], turnOrder[j] = turnOrder[j], turnOrder[i]
	})
	turnOrder = append([]string{r.Owner}, turnOrder...)

	// Create game engine
	resetTimer := func() {
		if r.Timer != nil {
			r.Timer.Reset()
		}
	}
	r.Engine = NewGameEngine(r.Settings, turnOrder, resetTimer)

	// Sync player connection-level state
	for name, p := range r.Players {
		if ps, ok := r.Engine.Players[name]; ok {
			p.Score = ps.Score
			p.Lives = ps.Lives
		}
	}

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


// ValidateAndSubmitWord delegates to GameEngine for word validation and submission.
func (r *Room) ValidateAndSubmitWord(word, playerName string) (ValidateResult, string) {
	r.mu.Lock()
	if r.Status != "playing" || r.Engine == nil {
		r.mu.Unlock()
		return ValidateRejected, "ゲームが開始されていません"
	}
	r.mu.Unlock()

	hasVotePending := r.Votes != nil && r.Votes.HasPendingVote()
	result, msg := r.Engine.ValidateAndSubmitWord(word, playerName, hasVotePending)

	// Sync player state back to connection-level Player
	if result == ValidateOK || result == ValidatePenalty {
		r.syncPlayerState(playerName)
	}
	if result == ValidateOK && r.Votes != nil {
		r.Votes.Clear()
	}
	return result, msg
}

// syncPlayerState syncs GameEngine state back to the connection-level Player.
func (r *Room) syncPlayerState(playerName string) {
	if r.Engine == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Engine.mu.Lock()
	defer r.Engine.mu.Unlock()
	if ps, ok := r.Engine.Players[playerName]; ok {
		if p, ok2 := r.Players[playerName]; ok2 {
			p.Score = ps.Score
			p.Lives = ps.Lives
		}
	}
}

// syncAllPlayerState syncs all player states from Engine to Room.
func (r *Room) syncAllPlayerState() {
	if r.Engine == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Engine.mu.Lock()
	defer r.Engine.mu.Unlock()
	for name, ps := range r.Engine.Players {
		if p, ok := r.Players[name]; ok {
			p.Score = ps.Score
			p.Lives = ps.Lives
		}
	}
}

// checkElimination delegates to GameEngine.
func (r *Room) checkElimination(playerName string) (eliminated bool, gameOver bool, lastSurvivor string) {
	r.mu.Lock()
	totalPlayers := len(r.Players)
	r.mu.Unlock()
	return r.Engine.CheckElimination(playerName, totalPlayers)
}

// getLivesLocked returns lives from GameEngine. Caller must hold r.mu.
func (r *Room) getLivesLocked() map[string]int {
	if r.Engine != nil {
		return r.Engine.GetLives()
	}
	lives := make(map[string]int, len(r.Players))
	for name, p := range r.Players {
		lives[name] = p.Lives
	}
	return lives
}

// GetLives returns a map of player lives.
func (r *Room) GetLives() map[string]int {
	if r.Engine != nil {
		return r.Engine.GetLives()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	lives := make(map[string]int, len(r.Players))
	for name, p := range r.Players {
		lives[name] = p.Lives
	}
	return lives
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
			pv := r.Votes.GetPending()
			if pv != nil && r.Engine != nil {
				r.Engine.ApplyWord(pv.Word, pv.Hiragana, pv.Player)
				r.syncPlayerState(pv.Player)
			}
		}
		r.Votes.Clear()
		return
	}

	// Challenge vote
	if result.Accepted || !result.Reverted {
		return
	}

	// Challenge rejected — revert the word via Engine
	if r.Engine != nil {
		r.Engine.RevertWord(result.Word, result.Player)
		r.syncPlayerState(result.Player)
	}
}

// StartChallengeVote starts a vote to challenge the last word.
func (r *Room) StartChallengeVote(challengerName string) (VoteInfo, error) {
	r.mu.Lock()
	if r.Status != "playing" || r.Engine == nil {
		r.mu.Unlock()
		return VoteInfo{}, fmt.Errorf("ゲームが開始されていません")
	}
	r.mu.Unlock()

	history, _, _, _ := r.Engine.Snapshot()
	if len(history) == 0 {
		return VoteInfo{}, fmt.Errorf("まだ単語がありません")
	}
	last := history[len(history)-1]

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
	if r.Engine != nil {
		return r.Engine.GetScores()
	}
	scores := make(map[string]int, len(r.Players))
	for name, p := range r.Players {
		scores[name] = p.Score
	}
	return scores
}

// GetScores returns a map of player scores.
func (r *Room) GetScores() map[string]int {
	if r.Engine != nil {
		return r.Engine.GetScores()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	scores := make(map[string]int, len(r.Players))
	for name, p := range r.Players {
		scores[name] = p.Score
	}
	return scores
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

	scores := r.getScoresLocked()
	players := make([]map[string]any, 0, len(r.Players))
	for name := range r.Players {
		players = append(players, map[string]any{
			"name":  name,
			"score": scores[name],
		})
	}

	var history []WordEntry
	var currentWord string
	var turnOrder []string
	var currentTurn string
	if r.Engine != nil {
		var turnIndex int
		history, currentWord, turnOrder, turnIndex = r.Engine.Snapshot()
		if len(turnOrder) > 0 && turnIndex < len(turnOrder) {
			currentTurn = turnOrder[turnIndex]
		}
	}

	state := map[string]any{
		"type":        "room_state",
		"roomId":      r.ID,
		"settings":    r.Settings,
		"players":     players,
		"history":     history,
		"currentWord": currentWord,
		"status":      r.Status,
	}

	if r.Settings.TimeLimit > 0 && r.Timer != nil {
		state["timeLeft"] = r.Timer.TimeLeft()
	}
	state["turnOrder"] = turnOrder
	if currentTurn != "" {
		state["currentTurn"] = currentTurn
	}
	state["owner"] = r.Owner
	state["lives"] = r.getLivesLocked()
	maxLives := r.Settings.MaxLives
	if maxLives <= 0 {
		maxLives = defaultMaxLives
	}
	state["maxLives"] = maxLives
	return state
}
