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

// HandleWS handles WebSocket connections for the game.
func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade", "error", err)
		return
	}

	var playerName string
	var currentRoom *Room
	var currentPlayer *Player

	// sendDirect writes a message directly to the WebSocket connection.
	// Only safe to use BEFORE writePump is started (i.e., before joining a room).
	sendDirect := func(v any) {
		conn.WriteJSON(v)
	}

	// sendToPlayer sends a message via the player's Send channel.
	// Safe to use after writePump is started.
	sendToPlayer := func(v any) {
		if currentPlayer == nil {
			return
		}
		data := mustMarshal(v)
		select {
		case currentPlayer.Send <- data:
		default:
			// drop if channel full
		}
	}

	// sendMsg sends a message using the appropriate method based on current state.
	sendMsg := func(v any) {
		if currentPlayer != nil {
			sendToPlayer(v)
		} else {
			sendDirect(v)
		}
	}

	sendErr := func(message string) {
		sendMsg(map[string]any{
			"type":    "error",
			"message": message,
		})
	}

	// leaveCurrentRoom removes the player from their current room.
	leaveCurrentRoom := func() {
		if currentRoom == nil || playerName == "" {
			return
		}
		remaining := currentRoom.RemovePlayer(playerName)
		s.Rooms.UntrackPlayer(playerName)

		currentRoom.Broadcast(mustMarshal(map[string]any{
			"type":   "player_left",
			"player": playerName,
		}))

		currentRoom.Broadcast(mustMarshal(map[string]any{
			"type":    "player_list",
			"players": currentRoom.PlayerNames(),
		}))

		if remaining == 0 {
			currentRoom.StopTimer()
			s.Rooms.RemoveRoom(currentRoom.ID)
			slog.Info("room removed (empty)", "roomId", currentRoom.ID)
		}
		currentRoom = nil
		currentPlayer = nil
	}

	// Cleanup on disconnect
	defer func() {
		leaveCurrentRoom()
		conn.Close()
	}()

	for {
		var msg WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("websocket read", "error", err)
			}
			return
		}

		switch msg.Type {
		case "get_rooms":
			rooms := s.Rooms.ListRooms()
			if rooms == nil {
				rooms = []RoomInfo{}
			}
			sendMsg(map[string]any{
				"type":  "rooms",
				"rooms": rooms,
			})

		case "get_genres":
			sendMsg(map[string]any{
				"type":     "genres",
				"kanaRows": GetKanaRowNames(),
			})

		case "create_room":
			if msg.Name == "" || msg.Settings == nil {
				sendErr("名前とルーム設定が必要です")
				continue
			}
			// Check if this name is already in a room (from another connection)
			if existingRoomID := s.Rooms.PlayerRoomID(msg.Name); existingRoomID != "" {
				// Only allow if this is the same connection & same player name (re-creating)
				if playerName != msg.Name || currentRoom == nil || currentRoom.ID != existingRoomID {
					sendErr(fmt.Sprintf("「%s」は既に別のルームに参加しています", msg.Name))
					continue
				}
			}
			// Leave current room first if in one
			leaveCurrentRoom()
			playerName = msg.Name
			room, player := s.handleCreateRoom(conn, playerName, msg.Settings)
			currentRoom = room
			currentPlayer = player
			s.Rooms.TrackPlayer(playerName, currentRoom.ID)
			go writePump(conn, currentPlayer)

		case "join":
			if msg.Name == "" || msg.RoomID == "" {
				sendErr("名前とルームIDが必要です")
				continue
			}
			// Check if this name is already in a room (from another connection)
			if existingRoomID := s.Rooms.PlayerRoomID(msg.Name); existingRoomID != "" {
				// Only allow if this is the same connection & same player name (re-joining)
				if playerName != msg.Name || currentRoom == nil || currentRoom.ID != existingRoomID {
					sendErr(fmt.Sprintf("「%s」は既に別のルームに参加しています", msg.Name))
					continue
				}
			}
			// Leave current room first if in one
			leaveCurrentRoom()
			playerName = msg.Name
			room, player, err := s.handleJoinRoom(conn, playerName, msg.RoomID)
			if err != nil {
				sendErr(err.Error())
				continue
			}
			currentRoom = room
			currentPlayer = player
			s.Rooms.TrackPlayer(playerName, currentRoom.ID)
			go writePump(conn, currentPlayer)

		case "leave_room":
			leaveCurrentRoom()

		case "start_game":
			if currentRoom == nil {
				sendErr("ルームに参加していません")
				continue
			}
			if currentRoom.Owner != playerName {
				sendErr("ゲームを開始できるのはルーム作成者のみです")
				continue
			}
			if msg.Settings != nil {
				if err := currentRoom.UpdateSettings(*msg.Settings); err != nil {
					sendErr(err.Error())
					continue
				}
				// Broadcast updated settings to all players
				currentRoom.Broadcast(mustMarshal(map[string]any{
					"type":     "settings_updated",
					"settings": currentRoom.Settings,
				}))
			}
			s.handleStartGame(currentRoom)

		case "answer":
			if currentRoom == nil || playerName == "" {
				sendErr("ルームに参加していません")
				continue
			}
			s.handleAnswer(currentRoom, playerName, msg.Word)

		case "vote":
			if currentRoom == nil || playerName == "" {
				sendErr("ルームに参加していません")
				continue
			}
			if msg.Accept == nil {
				sendErr("投票内容が必要です")
				continue
			}
			s.handleVote(currentRoom, playerName, *msg.Accept)

		case "challenge":
			if currentRoom == nil || playerName == "" {
				sendErr("ルームに参加していません")
				continue
			}
			s.handleChallenge(currentRoom, playerName)

		case "rebuttal":
			if currentRoom == nil || playerName == "" {
				sendErr("ルームに参加していません")
				continue
			}
			if msg.Rebuttal == "" {
				sendErr("反論メッセージが必要です")
				continue
			}
			s.handleRebuttal(currentRoom, playerName, msg.Rebuttal)

		case "withdraw_challenge":
			if currentRoom == nil || playerName == "" {
				sendErr("ルームに参加していません")
				continue
			}
			s.handleWithdrawChallenge(currentRoom, playerName)

		default:
			sendErr(fmt.Sprintf("unknown message type: %s", msg.Type))
		}
	}
}

// writePump pumps messages from the player's Send channel to the WebSocket.
func writePump(conn *websocket.Conn, p *Player) {
	if p == nil {
		return
	}
	for msg := range p.Send {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (s *Server) handleCreateRoom(conn *websocket.Conn, name string, settings *RoomSettings) (*Room, *Player) {
	roomID := generateRoomID()
	room := s.Rooms.CreateRoom(roomID, *settings)
	room.Owner = name
	room.OnGameOver = s.makeGameOverCallback()

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
	if isPlaying {
		turnOrder := make([]string, len(room.TurnOrder))
		copy(turnOrder, room.TurnOrder)
		currentTurn := ""
		if len(room.TurnOrder) > 0 && room.TurnIndex < len(room.TurnOrder) {
			currentTurn = room.TurnOrder[room.TurnIndex]
		}
		lives := room.getLivesLocked()
		maxLives := room.Settings.MaxLives
		if maxLives <= 0 {
			maxLives = 3
		}
		scores := room.getScoresLocked()
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

	room.mu.Lock()
	currentTurn := ""
	if len(room.TurnOrder) > 0 {
		currentTurn = room.TurnOrder[room.TurnIndex]
	}
	turnOrder := make([]string, len(room.TurnOrder))
	copy(turnOrder, room.TurnOrder)
	lives := room.getLivesLocked()
	maxLives := room.Settings.MaxLives
	if maxLives <= 0 {
		maxLives = 3
	}
	room.mu.Unlock()

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
		room.mu.Lock()
		voteCount := 0
		voteReason := ""
		voteType := ""
		if room.pendingVote != nil {
			voteCount = len(room.pendingVote.Votes)
			voteReason = room.pendingVote.Reason
			voteType = room.pendingVote.Type
		}
		eligibleVoters := room.countEligibleVotersLocked()
		room.mu.Unlock()

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
			time.Sleep(15 * time.Second)
			resolved, result := room.ForceResolveVote()
			if resolved {
				s.broadcastVoteResult(room, result)
			}
		}()

	case ValidateOK:
		s.broadcastWordAccepted(room, word, playerName)

	case ValidatePenalty:
		// Word NOT accepted, but player loses a life
		room.mu.Lock()
		var livesLeft int
		var eliminated, gameOver bool
		var lastSurvivor string
		if p, ok := room.Players[playerName]; ok {
			livesLeft = p.Lives
		}
		eliminated, gameOver, lastSurvivor = room.checkElimination(playerName)
		lives := room.getLivesLocked()
		scores := room.getScoresLocked()
		history := room.History
		room.mu.Unlock()

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
			room.pendingVote = nil
			room.mu.Unlock()
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
	room.mu.Lock()
	var voteCount int
	if room.pendingVote != nil {
		voteCount = len(room.pendingVote.Votes)
	}
	eligibleVoters := room.countEligibleVotersLocked()
	room.mu.Unlock()

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
	room.mu.Lock()
	// Only the challenged player (the word submitter) can send a rebuttal
	if room.pendingVote == nil || room.pendingVote.Resolved || room.pendingVote.Type != "challenge" {
		room.mu.Unlock()
		return
	}
	if room.pendingVote.Player != playerName {
		room.mu.Unlock()
		return
	}
	room.mu.Unlock()

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
		time.Sleep(15 * time.Second)
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

	room.mu.Lock()
	nextTurn := ""
	if len(room.TurnOrder) > 0 {
		nextTurn = room.TurnOrder[room.TurnIndex]
	}
	lives := room.getLivesLocked()
	scores := room.getScoresLocked()
	history := make([]WordEntry, len(room.History))
	copy(history, room.History)
	currentWord := room.CurrentWord

	// Check if the penalized player is eliminated / game over
	var penaltyLivesLeft int
	if p, ok := room.Players[result.Player]; ok {
		penaltyLivesLeft = p.Lives
	}
	eliminated, gameOver, lastSurvivor := room.checkElimination(result.Player)
	room.mu.Unlock()

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
		room.pendingVote = nil
		room.mu.Unlock()
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
	room.mu.Lock()
	nextTurn := ""
	if len(room.TurnOrder) > 0 {
		nextTurn = room.TurnOrder[room.TurnIndex]
	}
	lives := room.getLivesLocked()
	room.mu.Unlock()

	room.Broadcast(mustMarshal(map[string]any{
		"type":        "word_accepted",
		"word":        word,
		"player":      playerName,
		"currentWord": word,
		"scores":      room.GetScores(),
		"history":     room.History,
		"currentTurn": nextTurn,
		"lives":       lives,
	}))
}
