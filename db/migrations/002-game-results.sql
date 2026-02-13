-- Game results table for sharing
CREATE TABLE IF NOT EXISTS game_results (
    id TEXT PRIMARY KEY,
    room_name TEXT NOT NULL,
    genre TEXT NOT NULL DEFAULT '',
    winner TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    scores_json TEXT NOT NULL DEFAULT '{}',
    history_json TEXT NOT NULL DEFAULT '[]',
    lives_json TEXT NOT NULL DEFAULT '{}',
    player_count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO migrations (migration_number, migration_name)
VALUES (002, '002-game-results');
