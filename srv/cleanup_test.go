package srv

import (
	"testing"
	"time"
)

func TestRoomEmptySinceSetOnLastPlayerLeave(t *testing.T) {
	rm := NewRoomManager()
	room := rm.CreateRoom("r1", RoomSettings{Name: "test"})

	p := &Player{Name: "alice", Send: make(chan []byte, 256)}
	room.AddPlayer(p)

	room.mu.Lock()
	if room.EmptySince != nil {
		t.Fatal("expected EmptySince to be nil when room has players")
	}
	room.mu.Unlock()

	remaining := room.RemovePlayer("alice")
	if remaining != 0 {
		t.Fatalf("expected 0 remaining, got %d", remaining)
	}

	// Simulate what ws.go does when room becomes empty
	now := time.Now()
	room.mu.Lock()
	room.EmptySince = &now
	room.mu.Unlock()

	room.mu.Lock()
	if room.EmptySince == nil {
		t.Fatal("expected EmptySince to be set after last player left")
	}
	room.mu.Unlock()
}

func TestRoomEmptySinceClearedOnPlayerJoin(t *testing.T) {
	rm := NewRoomManager()
	room := rm.CreateRoom("r1", RoomSettings{Name: "test"})

	now := time.Now()
	room.EmptySince = &now

	p := &Player{Name: "bob", Send: make(chan []byte, 256)}
	room.AddPlayer(p)

	room.mu.Lock()
	if room.EmptySince != nil {
		t.Fatal("expected EmptySince to be nil after player joined")
	}
	room.mu.Unlock()
}

func TestCleanupRemovesOldEmptyRooms(t *testing.T) {
	rm := NewRoomManager()

	room1 := rm.CreateRoom("old-empty", RoomSettings{Name: "old"})
	past := time.Now().Add(-10 * time.Minute)
	room1.EmptySince = &past

	room2 := rm.CreateRoom("new-empty", RoomSettings{Name: "new"})
	recent := time.Now().Add(-1 * time.Minute)
	room2.EmptySince = &recent

	room3 := rm.CreateRoom("active", RoomSettings{Name: "active"})
	room3.AddPlayer(&Player{Name: "alice", Send: make(chan []byte, 256)})

	rm.cleanupEmptyRooms(5 * time.Minute)

	if rm.GetRoom("old-empty") != nil {
		t.Error("expected old-empty room to be removed")
	}
	if rm.GetRoom("new-empty") == nil {
		t.Error("expected new-empty room to still exist")
	}
	if rm.GetRoom("active") == nil {
		t.Error("expected active room to still exist")
	}
}

func TestCleanupGoroutineStops(t *testing.T) {
	rm := NewRoomManager()
	rm.StartCleanup(50*time.Millisecond, 5*time.Minute)

	room := rm.CreateRoom("stale", RoomSettings{Name: "stale"})
	past := time.Now().Add(-10 * time.Minute)
	room.EmptySince = &past

	time.Sleep(200 * time.Millisecond)

	if rm.GetRoom("stale") != nil {
		t.Error("expected stale room to be cleaned up by goroutine")
	}

	rm.StopCleanup()
}

func TestMaxPlayersLimit(t *testing.T) {
	room := &Room{
		Settings: RoomSettings{MaxPlayers: 3},
		Players:  make(map[string]*Player),
	}
	if room.MaxPlayersLimit() != 3 {
		t.Errorf("expected MaxPlayersLimit=3, got %d", room.MaxPlayersLimit())
	}

	room2 := &Room{
		Settings: RoomSettings{},
		Players:  make(map[string]*Player),
	}
	if room2.MaxPlayersLimit() != defaultMaxPlayers {
		t.Errorf("expected MaxPlayersLimit=%d, got %d", defaultMaxPlayers, room2.MaxPlayersLimit())
	}
}

func TestMaxPlayersEnforcedInJoinRoom(t *testing.T) {
	rm := NewRoomManager()
	room := rm.CreateRoom("r1", RoomSettings{Name: "test", MaxPlayers: 2})

	room.AddPlayer(&Player{Name: "p1", Send: make(chan []byte, 256)})
	room.AddPlayer(&Player{Name: "p2", Send: make(chan []byte, 256)})

	room.mu.Lock()
	playerCount := len(room.Players)
	maxP := room.MaxPlayersLimit()
	room.mu.Unlock()

	if playerCount < maxP {
		t.Errorf("expected room to be at capacity, got %d/%d", playerCount, maxP)
	}
}

func TestListRoomsIncludesMaxPlayers(t *testing.T) {
	rm := NewRoomManager()
	rm.CreateRoom("r1", RoomSettings{Name: "test", MaxPlayers: 4})
	rm.CreateRoom("r2", RoomSettings{Name: "test2"})

	list := rm.ListRooms()
	if len(list) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(list))
	}

	for _, info := range list {
		if info.ID == "r1" && info.MaxPlayers != 4 {
			t.Errorf("expected r1 MaxPlayers=4, got %d", info.MaxPlayers)
		}
		if info.ID == "r2" && info.MaxPlayers != defaultMaxPlayers {
			t.Errorf("expected r2 MaxPlayers=%d, got %d", defaultMaxPlayers, info.MaxPlayers)
		}
	}
}
