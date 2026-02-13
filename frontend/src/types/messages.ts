// === Outgoing messages (client → server) ===
export type OutgoingMessage =
  | { type: 'create_room'; name: string; settings: RoomSettings }
  | { type: 'join'; name: string; roomId: string }
  | { type: 'start_game'; settings?: RoomSettings }
  | { type: 'answer'; word: string }
  | { type: 'leave_room' }
  | { type: 'get_rooms' }
  | { type: 'get_genres' }
  | { type: 'challenge' }
  | { type: 'vote'; accept: boolean }
  | { type: 'withdraw_challenge' }
  | { type: 'rebuttal'; rebuttal: string }
  | { type: 'update_settings'; settings: RoomSettings };

// === Incoming messages (server → client) ===
export type IncomingMessage =
  | { type: 'rooms'; rooms: RoomInfo[] }
  | { type: 'genres'; kanaRows: string[] }
  | { type: 'room_joined'; roomId: string; owner: string; settings: RoomSettings; players: PlayerInfo[]; scores: Record<string, number>; lives: Record<string, number>; maxLives: number; history: HistoryEntry[]; turnOrder: string[]; currentTurn: string; currentWord: string; status: string }
  | { type: 'room_state'; roomId: string; owner: string; settings: RoomSettings; players: PlayerInfo[]; scores: Record<string, number>; lives: Record<string, number>; maxLives: number; history: HistoryEntry[]; turnOrder: string[]; currentTurn: string; currentWord: string; status: string }
  | { type: 'player_joined'; player: string }
  | { type: 'player_left'; player: string }
  | { type: 'player_list'; players: string[] }
  | { type: 'game_started'; currentWord: string; firstWord: string; turnOrder: string[]; currentTurn: string; lives: Record<string, number>; maxLives: number; timeLimit: number }
  | { type: 'word_accepted'; word: string; player: string; scores: Record<string, number>; lives: Record<string, number>; currentTurn: string }
  | { type: 'answer_rejected'; message: string }
  | { type: 'timer'; timeLeft: number }
  | { type: 'game_over'; reason: string; winner?: string; loser?: string; scores: Record<string, number>; history: HistoryEntry[]; lives: Record<string, number>; resultId?: string }
  | { type: 'vote_request'; voteType: 'challenge' | 'genre'; word: string; player: string; challenger?: string; reason?: string; genre?: string; voteCount: number; totalPlayers: number }
  | { type: 'vote_update'; voteCount: number; totalPlayers: number }
  | { type: 'vote_result'; accepted: boolean; word: string; message?: string; reverted?: boolean; currentWord?: string; history?: HistoryEntry[]; scores?: Record<string, number>; lives?: Record<string, number>; currentTurn?: string; penaltyPlayer?: string; penaltyLives?: number; eliminated?: boolean }
  | { type: 'rebuttal'; player: string; rebuttal: string }
  | { type: 'challenge_withdrawn'; message?: string }
  | { type: 'penalty'; player: string; lives: number; reason: string; eliminated: boolean; allLives: Record<string, number> }
  | { type: 'turn_update'; turnOrder: string[]; currentTurn: string; scores: Record<string, number>; lives: Record<string, number>; maxLives: number }
  | { type: 'settings_updated'; settings: RoomSettings }
  | { type: 'error'; message: string };

// === Shared types ===
export interface RoomSettings {
  name: string;
  minLen: number;
  maxLen: number;
  genre: string;
  timeLimit: number;
  maxLives: number;
  allowedRows?: string[];
  noDakuten?: boolean;
  private?: boolean;
}

export interface RoomInfo {
  id: string;
  name: string;
  status: string;
  playerCount: number;
  players: number;
  settings: RoomSettings;
}

export interface PlayerInfo {
  name: string;
  score: number;
  lives: number;
}

export interface HistoryEntry {
  word: string;
  player: string;
}
