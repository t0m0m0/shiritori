package srv

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

	id := generateResultID()
	scoresJSON, _ := json.Marshal(req.Scores)
	historyJSON, _ := json.Marshal(req.History)
	livesJSON, _ := json.Marshal(req.Lives)
	playerCount := len(req.Scores)
	if playerCount == 0 {
		playerCount = 1
	}

	_, err := s.DB.Exec(
		`INSERT INTO game_results (id, room_name, genre, winner, reason, scores_json, history_json, lives_json, player_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.RoomName, req.Genre, req.Winner, req.Reason,
		string(scoresJSON), string(historyJSON), string(livesJSON),
		playerCount, time.Now().UTC(),
	)
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, resultPageHTML, 
		esc(title), esc(title), esc(desc), esc(ogpURL), esc(pageURL),
		esc(title), esc(desc), esc(ogpURL),
		string(resultJSON))
}

func esc(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
