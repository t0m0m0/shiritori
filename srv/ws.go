package srv

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"

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

	// Response fields
	Success bool        `json:"success,omitempty"`
	Message string      `json:"message,omitempty"`
	Rooms   []RoomInfo  `json:"rooms,omitempty"`
	Genres  []string    `json:"genres,omitempty"`
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

	// Cleanup on disconnect
	defer func() {
		if currentRoom != nil && playerName != "" {
			remaining := currentRoom.RemovePlayer(playerName)

			// Notify others
			currentRoom.Broadcast(mustMarshal(map[string]any{
				"type":   "player_left",
				"player": playerName,
			}))

			// Remove room if empty
			if remaining == 0 {
				currentRoom.StopTimer()
				s.Rooms.RemoveRoom(currentRoom.ID)
				slog.Info("room removed (empty)", "roomId", currentRoom.ID)
			}
		}
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
			s.handleGetRooms(conn)

		case "get_genres":
			s.handleGetGenres(conn)

		case "create_room":
			if msg.Name == "" || msg.Settings == nil {
				sendError(conn, "名前とルーム設定が必要です")
				continue
			}
			playerName = msg.Name
			room, player := s.handleCreateRoom(conn, playerName, msg.Settings)
			currentRoom = room
			_ = player
			// Start writer goroutine
			go writePump(conn, s.Rooms.GetRoom(currentRoom.ID).Players[playerName])

		case "join":
			if msg.Name == "" || msg.RoomID == "" {
				sendError(conn, "名前とルームIDが必要です")
				continue
			}
			playerName = msg.Name
			room, err := s.handleJoinRoom(conn, playerName, msg.RoomID)
			if err != nil {
				sendError(conn, err.Error())
				continue
			}
			currentRoom = room
			// Start writer goroutine
			go writePump(conn, currentRoom.Players[playerName])

		case "start_game":
			if currentRoom == nil {
				sendError(conn, "ルームに参加していません")
				continue
			}
			s.handleStartGame(currentRoom)

		case "answer":
			if currentRoom == nil || playerName == "" {
				sendError(conn, "ルームに参加していません")
				continue
			}
			s.handleAnswer(currentRoom, playerName, msg.Word)

		default:
			sendError(conn, fmt.Sprintf("unknown message type: %s", msg.Type))
		}
	}
}

func sendError(conn *websocket.Conn, message string) {
	conn.WriteJSON(map[string]any{
		"type":    "error",
		"message": message,
	})
}

func sendJSON(conn *websocket.Conn, v any) {
	conn.WriteJSON(v)
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

func (s *Server) handleGetRooms(conn *websocket.Conn) {
	rooms := s.Rooms.ListRooms()
	if rooms == nil {
		rooms = []RoomInfo{}
	}
	sendJSON(conn, map[string]any{
		"type":  "rooms",
		"rooms": rooms,
	})
}

func (s *Server) handleGetGenres(conn *websocket.Conn) {
	sendJSON(conn, map[string]any{
		"type":   "genres",
		"genres": getGenreList(),
	})
}

func (s *Server) handleCreateRoom(conn *websocket.Conn, name string, settings *RoomSettings) (*Room, *Player) {
	roomID := generateRoomID()
	room := s.Rooms.CreateRoom(roomID, *settings)

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

	return room, player
}

func (s *Server) handleJoinRoom(conn *websocket.Conn, name, roomID string) (*Room, error) {
	room := s.Rooms.GetRoom(roomID)
	if room == nil {
		return nil, fmt.Errorf("ルームが見つかりません: %s", roomID)
	}

	room.mu.Lock()
	if _, exists := room.Players[name]; exists {
		room.mu.Unlock()
		return nil, fmt.Errorf("名前「%s」はすでに使われています", name)
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

	return room, nil
}

func (s *Server) handleStartGame(room *Room) {
	word, err := room.StartGame()
	if err != nil {
		slog.Warn("start game failed", "error", err)
		return
	}

	hiragana := toHiragana(word)
	nextChar := getLastChar(hiragana)

	room.Broadcast(mustMarshal(map[string]any{
		"type":        "game_started",
		"currentWord": word,
		"nextChar":    string(nextChar),
		"history":     room.History,
		"timeLimit":   room.Settings.TimeLimit,
	}))
}

func (s *Server) handleAnswer(room *Room, playerName, word string) {
	ok, msg := room.ValidateAndSubmitWord(word, playerName)

	if !ok {
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
		return
	}

	// Success — broadcast to all
	hiragana := toHiragana(word)
	nextChar := getLastChar(hiragana)

	room.Broadcast(mustMarshal(map[string]any{
		"type":        "word_accepted",
		"word":        word,
		"player":      playerName,
		"nextChar":    string(nextChar),
		"currentWord": word,
		"scores":      room.GetScores(),
		"history":     room.History,
	}))
}
