package srv

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// GameResult is the stored result of a game.
type GameResult struct {
	ID          string         `json:"id"`
	RoomName    string         `json:"roomName"`
	Genre       string         `json:"genre"`
	Winner      string         `json:"winner"`
	Reason      string         `json:"reason"`
	Scores      map[string]int `json:"scores"`
	History     []WordEntry    `json:"history"`
	Lives       map[string]int `json:"lives"`
	PlayerCount int            `json:"playerCount"`
	CreatedAt   time.Time      `json:"createdAt"`
}

func generateResultID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// makeGameOverCallback returns a callback that saves the game result to DB
// and adds the resultId to the game_over message.
func (s *Server) makeGameOverCallback() func(room *Room, msg map[string]any) map[string]any {
	return func(room *Room, msg map[string]any) map[string]any {
		roomName := room.Settings.Name
		genre := room.Settings.Genre
		winner, _ := msg["winner"].(string)
		reason, _ := msg["reason"].(string)

		var scores map[string]int
		if s, ok := msg["scores"].(map[string]int); ok {
			scores = s
		}
		var history []WordEntry
		if h, ok := msg["history"].([]WordEntry); ok {
			history = h
		}
		var lives map[string]int
		if l, ok := msg["lives"].(map[string]int); ok {
			lives = l
		}

		id, err := s.saveGameResult(roomName, genre, winner, reason, scores, history, lives)
		if err != nil {
			slog.Error("save game result on game_over", "error", err)
		} else {
			msg["resultId"] = id
		}
		return msg
	}
}

// saveGameResult saves a game result to the DB and returns the result ID.
// Called server-side when a game ends, so only one save per game.
func (s *Server) saveGameResult(roomName, genre, winner, reason string, scores map[string]int, history []WordEntry, lives map[string]int) (string, error) {
	id := generateResultID()
	scoresJSON, _ := json.Marshal(scores)
	historyJSON, _ := json.Marshal(history)
	livesJSON, _ := json.Marshal(lives)
	playerCount := len(scores)
	if playerCount == 0 {
		playerCount = 1
	}
	_, err := s.DB.Exec(
		`INSERT INTO game_results (id, room_name, genre, winner, reason, scores_json, history_json, lives_json, player_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, roomName, genre, winner, reason,
		string(scoresJSON), string(historyJSON), string(livesJSON),
		playerCount, time.Now().UTC(),
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// HandleSaveResult saves a game result and returns the ID.
func (s *Server) HandleSaveResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RoomName string         `json:"roomName"`
		Genre    string         `json:"genre"`
		Winner   string         `json:"winner"`
		Reason   string         `json:"reason"`
		Scores   map[string]int `json:"scores"`
		History  []WordEntry    `json:"history"`
		Lives    map[string]int `json:"lives"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	id, err := s.saveGameResult(req.RoomName, req.Genre, req.Winner, req.Reason, req.Scores, req.History, req.Lives)
	if err != nil {
		slog.Error("save result", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// loadResult loads a game result from the database.
func (s *Server) loadResult(id string) (*GameResult, error) {
	var (
		result    GameResult
		scoresStr string
		histStr   string
		livesStr  string
	)
	err := s.DB.QueryRow(
		`SELECT id, room_name, genre, winner, reason, scores_json, history_json, lives_json, player_count, created_at
		 FROM game_results WHERE id = ?`, id,
	).Scan(&result.ID, &result.RoomName, &result.Genre, &result.Winner, &result.Reason,
		&scoresStr, &histStr, &livesStr, &result.PlayerCount, &result.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(scoresStr), &result.Scores)
	json.Unmarshal([]byte(histStr), &result.History)
	json.Unmarshal([]byte(livesStr), &result.Lives)
	return &result, nil
}

// resultPageData is the data passed to result.html template.
type resultPageData struct {
	Title       string
	Description string
	OGPURL      string
	PageURL     string
	ResultJSON  template.JS
}

// HandleViewResultPage serves the result page with OGP meta tags.
func (s *Server) HandleViewResultPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	result, err := s.loadResult(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	words := make([]string, len(result.History))
	for i, h := range result.History {
		words[i] = h.Word
	}
	chainText := strings.Join(words, " → ")
	if len([]rune(chainText)) > 80 {
		chainText = string([]rune(chainText)[:77]) + "…"
	}

	title := fmt.Sprintf("しりとり結果 - %d語のチェーン！", len(result.History))
	if result.Winner != "" {
		title = fmt.Sprintf("しりとり - %sさんの勝利！（%d語）", result.Winner, len(result.History))
	}

	desc := chainText
	if result.Genre != "" && result.Genre != "なし" {
		desc = fmt.Sprintf("[%s] %s", result.Genre, chainText)
	}

	scheme := "https"
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	ogpURL := fmt.Sprintf("%s/results/%s/ogp.svg", baseURL, id)
	pageURL := fmt.Sprintf("%s/results/%s", baseURL, id)

	resultJSON, _ := json.Marshal(result)

	tmpl, err := template.ParseFS(templatesFS, "templates/result.html")
	if err != nil {
		slog.Error("parse result template", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := resultPageData{
		Title:       title,
		Description: desc,
		OGPURL:      ogpURL,
		PageURL:     pageURL,
		ResultJSON:  template.JS(resultJSON),
	}
	if err := tmpl.Execute(w, data); err != nil {
		slog.Error("execute result template", "error", err)
	}
}
