package srv

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"

	"srv.exe.dev/db"
)

// Server holds shared state for the HTTP/WebSocket server.
type Server struct {
	DB           *sql.DB
	Hostname     string
	TemplatesDir string
	StaticDir    string
	Rooms        *RoomManager
}

// New creates a new Server with database and room manager.
func New(dbPath, hostname string) (*Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)
	srv := &Server{
		Hostname:     hostname,
		TemplatesDir: filepath.Join(baseDir, "templates"),
		StaticDir:    filepath.Join(baseDir, "static"),
		Rooms:        NewRoomManager(),
	}
	if err := srv.setUpDatabase(dbPath); err != nil {
		return nil, err
	}
	return srv, nil
}

// setUpDatabase initializes the database connection and runs migrations.
func (s *Server) setUpDatabase(dbPath string) error {
	wdb, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	s.DB = wdb
	if err := db.RunMigrations(wdb); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// HandleIndex serves the main page.
func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.TemplatesDir, "index.html"))
}

// HandleRoomInfo returns room summary data.
func (s *Server) HandleRoomInfo(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if roomID == "" {
		http.NotFound(w, r)
		return
	}
	room := s.Rooms.GetRoom(roomID)
	if room == nil {
		http.NotFound(w, r)
		return
	}
	players := room.PlayerNames()
	room.mu.Lock()
	payload := map[string]any{
		"id":          room.ID,
		"name":        room.Settings.Name,
		"owner":       room.Owner,
		"status":      room.Status,
		"playerCount": len(players),
		"settings":    room.Settings,
		"players":     players,
	}
	room.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// Serve starts the HTTP server with the configured routes.
func (s *Server) Serve(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.HandleIndex)
	mux.HandleFunc("GET /ws", s.HandleWS)
	mux.HandleFunc("GET /room/{id}", s.HandleRoomInfo)
	mux.HandleFunc("POST /api/results", s.HandleSaveResult)
	mux.HandleFunc("GET /results/{id}/ogp.svg", s.HandleOGPImage)
	mux.HandleFunc("GET /results/{id}", s.HandleViewResultPage)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.StaticDir))))
	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}
