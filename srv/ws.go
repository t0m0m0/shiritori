package srv

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// WSMessage is the envelope for all WebSocket messages.
type WSMessage struct {
	Type     string        `json:"type"`
	Name     string        `json:"name,omitempty"`
	RoomID   string        `json:"roomId,omitempty"`
	Word     string        `json:"word,omitempty"`
	Settings *RoomSettings `json:"settings,omitempty"`
	Accept   *bool         `json:"accept,omitempty"`    // for vote messages
	Reason   string        `json:"reason,omitempty"`    // for challenge
	Rebuttal string        `json:"rebuttal,omitempty"` // for challenged player's rebuttal

	// Response fields
	Success bool       `json:"success,omitempty"`
	Message string     `json:"message,omitempty"`
	Rooms   []RoomInfo `json:"rooms,omitempty"`
}

// mustMarshal marshals v to JSON or panics.
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("json marshal: %v", err))
	}
	return b
}

// generateRoomID creates a random 6-character room ID.
func generateRoomID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}

// WSConn holds per-connection state for a WebSocket client.
type WSConn struct {
	server        *Server
	conn          *websocket.Conn
	playerName    string
	currentRoom   *Room
	currentPlayer *Player
	rateLimiter   *ConnectionRateLimiter
}

// sendDirect writes a message directly to the WebSocket connection.
// Only safe to use BEFORE writePump is started (i.e., before joining a room).
func (wsc *WSConn) sendDirect(v any) {
	wsc.conn.WriteJSON(v)
}

// sendToPlayer sends a message via the player's Send channel.
// Safe to use after writePump is started.
func (wsc *WSConn) sendToPlayer(v any) {
	if wsc.currentPlayer == nil {
		return
	}
	data := mustMarshal(v)
	select {
	case wsc.currentPlayer.Send <- data:
	default:
		// drop if channel full
	}
}

// sendMsg sends a message using the appropriate method based on current state.
func (wsc *WSConn) sendMsg(v any) {
	if wsc.currentPlayer != nil {
		wsc.sendToPlayer(v)
	} else {
		wsc.sendDirect(v)
	}
}

func (wsc *WSConn) sendErr(message string) {
	wsc.sendMsg(map[string]any{
		"type":    "error",
		"message": message,
	})
}

// leaveCurrentRoom removes the player from their current room.
func (wsc *WSConn) leaveCurrentRoom() {
	if wsc.currentRoom == nil || wsc.playerName == "" {
		return
	}
	remaining := wsc.currentRoom.RemovePlayer(wsc.playerName)
	wsc.server.Rooms.UntrackPlayer(wsc.playerName)

	wsc.currentRoom.Broadcast(mustMarshal(map[string]any{
		"type":   "player_left",
		"player": wsc.playerName,
	}))

	wsc.currentRoom.Broadcast(mustMarshal(map[string]any{
		"type":    "player_list",
		"players": wsc.currentRoom.PlayerNames(),
	}))

	if remaining == 0 {
		wsc.currentRoom.StopTimer()
		now := time.Now()
		wsc.currentRoom.mu.Lock()
		wsc.currentRoom.EmptySince = &now
		wsc.currentRoom.mu.Unlock()
		slog.Info("room now empty, scheduled for cleanup", "roomId", wsc.currentRoom.ID)
	}
	wsc.currentRoom = nil
	wsc.currentPlayer = nil
}

func (wsc *WSConn) handleGetRooms(msg WSMessage) {
	rooms := wsc.server.Rooms.ListRooms()
	if rooms == nil {
		rooms = []RoomInfo{}
	}
	wsc.sendMsg(map[string]any{
		"type":  "rooms",
		"rooms": rooms,
	})
}

func (wsc *WSConn) handleGetGenres(msg WSMessage) {
	wsc.sendMsg(map[string]any{
		"type":     "genres",
		"kanaRows": GetKanaRowNames(),
	})
}

func (wsc *WSConn) handleCreateRoom(msg WSMessage) {
	if msg.Name == "" || msg.Settings == nil {
		wsc.sendErr("名前とルーム設定が必要です")
		return
	}
	// Check if this name is already in a room (from another connection)
	if existingRoomID := wsc.server.Rooms.PlayerRoomID(msg.Name); existingRoomID != "" {
		// Only allow if this is the same connection & same player name (re-creating)
		if wsc.playerName != msg.Name || wsc.currentRoom == nil || wsc.currentRoom.ID != existingRoomID {
			wsc.sendErr(fmt.Sprintf("「%s」は既に別のルームに参加しています", msg.Name))
			return
		}
	}
	// Leave current room first if in one
	wsc.leaveCurrentRoom()
	wsc.playerName = msg.Name
	room, player := wsc.server.handleCreateRoom(wsc.conn, wsc.playerName, msg.Settings)
	wsc.currentRoom = room
	wsc.currentPlayer = player
	wsc.server.Rooms.TrackPlayer(wsc.playerName, wsc.currentRoom.ID)
	go writePump(wsc.conn, wsc.currentPlayer)
}

func (wsc *WSConn) handleJoin(msg WSMessage) {
	if msg.Name == "" || msg.RoomID == "" {
		wsc.sendErr("名前とルームIDが必要です")
		return
	}
	// Check if this name is already in a room (from another connection)
	if existingRoomID := wsc.server.Rooms.PlayerRoomID(msg.Name); existingRoomID != "" {
		// Only allow if this is the same connection & same player name (re-joining)
		if wsc.playerName != msg.Name || wsc.currentRoom == nil || wsc.currentRoom.ID != existingRoomID {
			wsc.sendErr(fmt.Sprintf("「%s」は既に別のルームに参加しています", msg.Name))
			return
		}
	}
	// Leave current room first if in one
	wsc.leaveCurrentRoom()
	wsc.playerName = msg.Name
	room, player, err := wsc.server.handleJoinRoom(wsc.conn, wsc.playerName, msg.RoomID)
	if err != nil {
		wsc.sendErr(err.Error())
		return
	}
	wsc.currentRoom = room
	wsc.currentPlayer = player
	wsc.server.Rooms.TrackPlayer(wsc.playerName, wsc.currentRoom.ID)
	go writePump(wsc.conn, wsc.currentPlayer)
}

func (wsc *WSConn) handleLeaveRoom(msg WSMessage) {
	wsc.leaveCurrentRoom()
}

func (wsc *WSConn) handleStartGame(msg WSMessage) {
	if wsc.currentRoom == nil {
		wsc.sendErr("ルームに参加していません")
		return
	}
	if wsc.currentRoom.Owner != wsc.playerName {
		wsc.sendErr("ゲームを開始できるのはルーム作成者のみです")
		return
	}
	if msg.Settings != nil {
		if err := wsc.currentRoom.UpdateSettings(*msg.Settings); err != nil {
			wsc.sendErr(err.Error())
			return
		}
		// Broadcast updated settings to all players
		wsc.currentRoom.Broadcast(mustMarshal(map[string]any{
			"type":     "settings_updated",
			"settings": wsc.currentRoom.Settings,
		}))
	}
	wsc.server.handleStartGame(wsc.currentRoom)
}

func (wsc *WSConn) handleAnswer(msg WSMessage) {
	if wsc.currentRoom == nil || wsc.playerName == "" {
		wsc.sendErr("ルームに参加していません")
		return
	}
	wsc.server.handleAnswer(wsc.currentRoom, wsc.playerName, msg.Word)
}

func (wsc *WSConn) handleVote(msg WSMessage) {
	if wsc.currentRoom == nil || wsc.playerName == "" {
		wsc.sendErr("ルームに参加していません")
		return
	}
	if msg.Accept == nil {
		wsc.sendErr("投票内容が必要です")
		return
	}
	wsc.server.handleVote(wsc.currentRoom, wsc.playerName, *msg.Accept)
}

func (wsc *WSConn) handleChallenge(msg WSMessage) {
	if wsc.currentRoom == nil || wsc.playerName == "" {
		wsc.sendErr("ルームに参加していません")
		return
	}
	wsc.server.handleChallenge(wsc.currentRoom, wsc.playerName)
}

func (wsc *WSConn) handleRebuttal(msg WSMessage) {
	if wsc.currentRoom == nil || wsc.playerName == "" {
		wsc.sendErr("ルームに参加していません")
		return
	}
	if msg.Rebuttal == "" {
		wsc.sendErr("反論メッセージが必要です")
		return
	}
	wsc.server.handleRebuttal(wsc.currentRoom, wsc.playerName, msg.Rebuttal)
}

func (wsc *WSConn) handleWithdrawChallenge(msg WSMessage) {
	if wsc.currentRoom == nil || wsc.playerName == "" {
		wsc.sendErr("ルームに参加していません")
		return
	}
	wsc.server.handleWithdrawChallenge(wsc.currentRoom, wsc.playerName)
}

func (wsc *WSConn) handlePing(msg WSMessage) {
	wsc.sendMsg(map[string]any{
		"type": "pong",
	})
}

// readLoop reads messages from the WebSocket and dispatches them to handlers.
func (wsc *WSConn) readLoop() {
	defer func() {
		wsc.leaveCurrentRoom()
		wsc.conn.Close()
	}()

	for {
		var msg WSMessage
		if err := wsc.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("websocket read", "error", err)
			}
			return
		}

		// Rate limit check
		allowed, shouldDisconnect := wsc.rateLimiter.Allow(msg.Type)
		if !allowed {
			if shouldDisconnect {
				slog.Warn("rate limit exceeded, disconnecting", "player", wsc.playerName, "type", msg.Type)
				wsc.sendErr("レート制限を超過しました。接続を切断します。")
				return
			}
			wsc.sendErr("操作が速すぎます。少し待ってからやり直してください。")
			continue
		}

		switch msg.Type {
		case "get_rooms":
			wsc.handleGetRooms(msg)
		case "get_genres":
			wsc.handleGetGenres(msg)
		case "create_room":
			wsc.handleCreateRoom(msg)
		case "join":
			wsc.handleJoin(msg)
		case "leave_room":
			wsc.handleLeaveRoom(msg)
		case "start_game":
			wsc.handleStartGame(msg)
		case "answer":
			wsc.handleAnswer(msg)
		case "vote":
			wsc.handleVote(msg)
		case "challenge":
			wsc.handleChallenge(msg)
		case "rebuttal":
			wsc.handleRebuttal(msg)
		case "withdraw_challenge":
			wsc.handleWithdrawChallenge(msg)
		case "ping":
			wsc.handlePing(msg)
		default:
			wsc.sendErr(fmt.Sprintf("unknown message type: %s", msg.Type))
		}
	}
}

// HandleWS handles WebSocket connections for the game.
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "error", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	wsc := &WSConn{
		server:      s,
		conn:        conn,
		rateLimiter: NewConnectionRateLimiter(),
	}
	wsc.readLoop()
}

// writePump pumps messages from the player's Send channel to the WebSocket.
func writePump(conn *websocket.Conn, p *Player) {
	if p == nil {
		return
	}
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case msg, ok := <-p.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleCreateRoom(conn *websocket.Conn, name string, settings *RoomSettings) (*Room, *Player) {
	roomID := generateRoomID()
	room := s.Rooms.CreateRoom(roomID, *settings)
	room.Owner = name
	room.OnGameOver = s.makeGameOverCallback()

	// Set up vote manager
	room.Votes = NewVoteManager(
		func(name string) bool {
			room.mu.Lock()
			defer room.mu.Unlock()
			_, ok := room.Players[name]
			return ok
		},
		func() int {
			room.mu.Lock()
			defer room.mu.Unlock()
			return len(room.Players)
		},
	)

	// Set up timer with callbacks
	room.Timer = NewTimerManager(
		func(timeLeft int) {
			room.Broadcast(mustMarshal(map[string]any{
				"type":     "timer",
				"timeLeft": timeLeft,
			}))
		},
		func() {
			room.mu.Lock()
			if room.Status != "playing" {
				room.mu.Unlock()
				return
			}
			room.Status = "finished"
			loser := ""
			if room.Engine != nil {
				loser = room.Engine.CurrentTurn()
			}
			var history []WordEntry
			if room.Engine != nil {
				history, _, _, _ = room.Engine.Snapshot()
			}
			gameOverMsg := map[string]any{
				"type":    "game_over",
				"reason":  "タイムアップ",
				"loser":   loser,
				"scores":  room.getScoresLocked(),
				"history": history,
				"lives":   room.getLivesLocked(),
			}
			if room.OnGameOver != nil {
				gameOverMsg = room.OnGameOver(room, gameOverMsg)
			}
			room.broadcastLocked(mustMarshal(gameOverMsg))
			room.mu.Unlock()
		},
	)

	player := &Player{
		Name: name,
		Conn: conn,
		Send: make(chan []byte, 256),
	}
	room.AddPlayer(player)

	slog.Info("room created", "roomId", roomID, "player", name, "roomName", settings.Name)

	// Send room state to creator
	state := room.GetState()
	state["type"] = "room_joined"
	player.Send <- mustMarshal(state)

	room.Broadcast(mustMarshal(map[string]any{
		"type":    "player_list",
		"players": room.PlayerNames(),
	}))

	return room, player
}

func (s *Server) handleJoinRoom(conn *websocket.Conn, name, roomID string) (*Room, *Player, error) {
	room := s.Rooms.GetRoom(roomID)
	if room == nil {
		return nil, nil, fmt.Errorf("ルームが見つかりません: %s", roomID)
	}

	room.mu.Lock()
	if _, exists := room.Players[name]; exists {
		room.mu.Unlock()
		return nil, nil, fmt.Errorf("名前「%s」はすでに使われています", name)
	}
	maxP := room.MaxPlayersLimit()
	if len(room.Players) >= maxP {
		room.mu.Unlock()
		return nil, nil, fmt.Errorf("ルームが満員です（最大%d人）", maxP)
	}
	room.mu.Unlock()

	player := &Player{
		Name: name,
		Conn: conn,
		Send: make(chan []byte, 256),
	}
	room.AddPlayer(player)

	slog.Info("player joined", "roomId", roomID, "player", name)

	// Send room state to new player
	state := room.GetState()
	state["type"] = "room_joined"
	player.Send <- mustMarshal(state)

	// Notify others
	room.Broadcast(mustMarshal(map[string]any{
		"type":   "player_joined",
		"player": name,
	}))

	room.Broadcast(mustMarshal(map[string]any{
		"type":    "player_list",
		"players": room.PlayerNames(),
	}))

	// If the game is already playing, broadcast updated turn order and lives to all
	room.mu.Lock()
	isPlaying := room.Status == "playing"
	if isPlaying && room.Engine != nil {
		_, _, turnOrder, turnIndex := room.Engine.Snapshot()
		currentTurn := ""
		if len(turnOrder) > 0 && turnIndex < len(turnOrder) {
			currentTurn = turnOrder[turnIndex]
		}
		lives := room.Engine.GetLives()
		maxLives := room.Engine.MaxLives()
		scores := room.Engine.GetScores()
		room.mu.Unlock()

		room.Broadcast(mustMarshal(map[string]any{
			"type":        "turn_update",
			"turnOrder":   turnOrder,
			"currentTurn": currentTurn,
			"lives":       lives,
			"maxLives":    maxLives,
			"scores":      scores,
		}))
	} else {
		room.mu.Unlock()
	}

	return room, player, nil
}

func (s *Server) handleStartGame(room *Room) {
	err := room.StartGame()
	if err != nil {
		slog.Warn("start game failed", "error", err)
		return
	}

	currentTurn := ""
	var turnOrder []string
	var lives map[string]int
	maxLives := defaultMaxLives
	if room.Engine != nil {
		currentTurn = room.Engine.CurrentTurn()
		_, _, turnOrder, _ = room.Engine.Snapshot()
		lives = room.Engine.GetLives()
		maxLives = room.Engine.MaxLives()
	}

	room.Broadcast(mustMarshal(map[string]any{
		"type":        "game_started",
		"currentWord": "",
		"history":     []WordEntry{},
		"timeLimit":   room.Settings.TimeLimit,
		"currentTurn": currentTurn,
		"turnOrder":   turnOrder,
		"lives":       lives,
		"maxLives":    maxLives,
	}))
}

func (s *Server) handleAnswer(room *Room, playerName, word string) {
	result, msg := room.ValidateAndSubmitWord(word, playerName)

	switch result {
	case ValidateRejected:
		// Send error only to the submitting player
		room.mu.Lock()
		if p, exists := room.Players[playerName]; exists {
			select {
			case p.Send <- mustMarshal(map[string]any{
				"type":    "answer_rejected",
				"word":    word,
				"message": msg,
			}):
			default:
			}
		}
		room.mu.Unlock()

	case ValidateVote:
		// Start vote — broadcast to all players
		voteCount := 0
		voteReason := ""
		voteType := ""
		if pv := room.Votes.GetPending(); pv != nil {
			voteCount = len(pv.Votes)
			voteReason = pv.Reason
			voteType = pv.Type
		}
		_, eligibleVoters := room.Votes.VoteCount()

		room.Broadcast(mustMarshal(map[string]any{
			"type":         "vote_request",
			"voteType":     voteType,
			"word":         word,
			"player":       playerName,
			"genre":        room.Settings.Genre,
			"message":      msg,
			"reason":       voteReason,
			"voteCount":    voteCount,
			"totalPlayers": eligibleVoters,
		}))

		// Start a 15-second vote timer
		go func() {
			time.Sleep(voteTimeout)
			resolved, result := room.ForceResolveVote()
			if resolved {
				s.broadcastVoteResult(room, result)
			}
		}()

	case ValidateOK:
		s.broadcastWordAccepted(room, word, playerName)

	case ValidatePenalty:
		// Word NOT accepted, but player loses a life
		livesLeft := 0
		if room.Engine != nil {
			livesLeft = room.Engine.GetPlayerLives(playerName)
		}
		room.mu.Lock()
		totalPlayers := len(room.Players)
		room.mu.Unlock()
		eliminated, gameOver, lastSurvivor := room.Engine.CheckElimination(playerName, totalPlayers)
		lives := room.Engine.GetLives()
		scores := room.Engine.GetScores()
		history, _, _, _ := room.Engine.Snapshot()

		room.Broadcast(mustMarshal(map[string]any{
			"type":       "penalty",
			"player":     playerName,
			"reason":     msg,
			"lives":      livesLeft,
			"eliminated": eliminated,
			"allLives":   lives,
		}))

		if gameOver {
			room.mu.Lock()
			room.Status = "finished"
			room.mu.Unlock()
			room.Votes.Clear()
			room.StopTimer()

			reason := "ゲーム終了"
			if lastSurvivor != "" {
				reason = fmt.Sprintf("%sさんの勝利！", lastSurvivor)
			}
			gameOverMsg := map[string]any{
				"type":    "game_over",
				"reason":  reason,
				"winner":  lastSurvivor,
				"scores":  scores,
				"history": history,
				"lives":   lives,
			}
			if room.OnGameOver != nil {
				gameOverMsg = room.OnGameOver(room, gameOverMsg)
			}
			room.Broadcast(mustMarshal(gameOverMsg))
		}
	}
}

func (s *Server) handleVote(room *Room, playerName string, accept bool) {
	resolved, result := room.CastVote(playerName, accept)

	// Broadcast vote progress
	voteCount, eligibleVoters := room.Votes.VoteCount()

	if !resolved {
		// Notify progress
		room.Broadcast(mustMarshal(map[string]any{
			"type":         "vote_update",
			"voteCount":    voteCount,
			"totalPlayers": eligibleVoters,
		}))
		return
	}

	s.broadcastVoteResult(room, result)
}

func (s *Server) handleRebuttal(room *Room, playerName, rebuttal string) {
	// Only the challenged player (the word submitter) can send a rebuttal
	pv := room.Votes.GetPending()
	if pv == nil || pv.Resolved || pv.Type != "challenge" {
		return
	}
	if pv.Player != playerName {
		return
	}

	// Broadcast the rebuttal to all players
	room.Broadcast(mustMarshal(map[string]any{
		"type":     "rebuttal",
		"player":   playerName,
		"rebuttal": rebuttal,
	}))
}

func (s *Server) handleWithdrawChallenge(room *Room, playerName string) {
	if !room.WithdrawChallenge(playerName) {
		room.mu.Lock()
		if p, ok := room.Players[playerName]; ok {
			select {
			case p.Send <- mustMarshal(map[string]any{
				"type":    "error",
				"message": "指摘を取り下げることができません",
			}):
			default:
			}
		}
		room.mu.Unlock()
		return
	}

	room.Broadcast(mustMarshal(map[string]any{
		"type":       "challenge_withdrawn",
		"challenger": playerName,
		"message":    fmt.Sprintf("%sさんが指摘を取り下げました", playerName),
	}))
}

func (s *Server) handleChallenge(room *Room, playerName string) {
	info, err := room.StartChallengeVote(playerName)
	if err != nil {
		// Send error via player's channel
		room.mu.Lock()
		if p, ok := room.Players[playerName]; ok {
			select {
			case p.Send <- mustMarshal(map[string]any{
				"type":    "error",
				"message": err.Error(),
			}):
			default:
			}
		}
		room.mu.Unlock()
		return
	}

	room.Broadcast(mustMarshal(map[string]any{
		"type":         "vote_request",
		"voteType":     info.Type,
		"word":         info.Word,
		"player":       info.Player,
		"challenger":   info.Challenger,
		"reason":       info.Reason,
		"voteCount":    info.VoteCount,
		"totalPlayers": info.Total,
	}))

	// Start a 15-second vote timer
	go func() {
		time.Sleep(voteTimeout)
		resolved, result := room.ForceResolveVote()
		if resolved {
			s.broadcastVoteResult(room, result)
		}
	}()
}

func (s *Server) broadcastVoteResult(room *Room, result VoteResolution) {
	if result.Type == "genre" {
		if result.Accepted {
			// Word accepted via vote — broadcast as normal word accepted
			room.Broadcast(mustMarshal(map[string]any{
				"type":     "vote_result",
				"voteType": result.Type,
				"word":     result.Word,
				"player":   result.Player,
				"accepted": true,
				"message":  fmt.Sprintf("投票により「%s」が承認されました！", result.Word),
			}))
			s.broadcastWordAccepted(room, result.Word, result.Player)
		} else {
			room.Broadcast(mustMarshal(map[string]any{
				"type":     "vote_result",
				"voteType": result.Type,
				"word":     result.Word,
				"player":   result.Player,
				"accepted": false,
				"message":  fmt.Sprintf("投票により「%s」は却下されました", result.Word),
			}))
		}
		return
	}

	// Challenge vote
	if result.Accepted {
		room.Broadcast(mustMarshal(map[string]any{
			"type":       "vote_result",
			"voteType":   result.Type,
			"word":       result.Word,
			"player":     result.Player,
			"challenger": result.Challenger,
			"accepted":   true,
			"message":    fmt.Sprintf("投票により「%s」は有効と認められました", result.Word),
		}))
		return
	}

	nextTurn := ""
	var lives map[string]int
	var scores map[string]int
	var history []WordEntry
	var currentWord string
	if room.Engine != nil {
		nextTurn = room.Engine.CurrentTurn()
		lives = room.Engine.GetLives()
		scores = room.Engine.GetScores()
		history, currentWord, _, _ = room.Engine.Snapshot()
	}

	// Check if the penalized player is eliminated / game over
	penaltyLivesLeft := 0
	if room.Engine != nil {
		penaltyLivesLeft = room.Engine.GetPlayerLives(result.Player)
	}
	room.mu.Lock()
	totalPlayers := len(room.Players)
	room.mu.Unlock()
	var eliminated, gameOver bool
	var lastSurvivor string
	if room.Engine != nil {
		eliminated, gameOver, lastSurvivor = room.Engine.CheckElimination(result.Player, totalPlayers)
	}

	room.Broadcast(mustMarshal(map[string]any{
		"type":        "vote_result",
		"voteType":    result.Type,
		"word":        result.Word,
		"player":      result.Player,
		"challenger":  result.Challenger,
		"accepted":    false,
		"reverted":    true,
		"currentWord": currentWord,
		"currentTurn": nextTurn,
		"lives":       lives,
		"scores":      scores,
		"history":     history,
		"penaltyPlayer": result.Player,
		"penaltyLives":  penaltyLivesLeft,
		"eliminated":    eliminated,
		"message":     fmt.Sprintf("投票により「%s」は却下されました。%sさんはライフ-1、もう一度入力してください", result.Word, result.Player),
	}))

	if gameOver {
		room.mu.Lock()
		room.Status = "finished"
		room.mu.Unlock()
		room.Votes.Clear()
		room.StopTimer()

		reason := "ゲーム終了"
		if lastSurvivor != "" {
			reason = fmt.Sprintf("%sさんの勝利！", lastSurvivor)
		}
		gameOverMsg := map[string]any{
			"type":    "game_over",
			"reason":  reason,
			"winner":  lastSurvivor,
			"scores":  scores,
			"history": history,
			"lives":   lives,
		}
		if room.OnGameOver != nil {
			gameOverMsg = room.OnGameOver(room, gameOverMsg)
		}
		room.Broadcast(mustMarshal(gameOverMsg))
	}
}

func (s *Server) broadcastWordAccepted(room *Room, word, playerName string) {
	nextTurn := ""
	var lives map[string]int
	var scores map[string]int
	var history []WordEntry
	if room.Engine != nil {
		nextTurn = room.Engine.CurrentTurn()
		lives = room.Engine.GetLives()
		scores = room.Engine.GetScores()
		history, _, _, _ = room.Engine.Snapshot()
	}

	room.Broadcast(mustMarshal(map[string]any{
		"type":        "word_accepted",
		"word":        word,
		"player":      playerName,
		"currentWord": word,
		"scores":      scores,
		"history":     history,
		"currentTurn": nextTurn,
		"lives":       lives,
	}))
}
