package srv

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"srv.exe.dev/db"
)

// Server holds shared state for the HTTP/WebSocket server.
type Server struct {
	DB       *sql.DB
	Hostname string
	Rooms    *RoomManager
}

// New creates a new Server with database and room manager.
func New(dbPath, hostname string) (*Server, error) {
	srv := &Server{
		Hostname: hostname,
		Rooms:    NewRoomManager(),
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

// HandleIndex serves the React SPA index.html.
func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/dist/index.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
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
	s.Rooms.StartCleanup(roomCleanupInterval, roomMaxEmptyAge)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.HandleIndex)
	mux.HandleFunc("GET /ws", s.HandleWS)
	mux.HandleFunc("GET /room/{id}", s.HandleRoomInfo)
	mux.HandleFunc("POST /api/results", s.HandleSaveResult)
	mux.HandleFunc("GET /results/{id}/ogp.svg", s.HandleOGPImage)
	mux.HandleFunc("GET /results/{id}", s.HandleViewResultPage)
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}
