import { useReducer } from 'react';
import type { RoomSettings, RoomInfo, HistoryEntry, IncomingMessage } from '../types/messages';

const DEFAULT_MAX_LIVES = 3;

export type Screen = 'lobby' | 'game';

export interface Toast {
  id: number;
  message: string;
  type?: 'error' | 'success' | 'info';
}

export interface GameState {
  screen: Screen;
  myName: string;
  // Lobby
  rooms: RoomInfo[];
  kanaRowNames: string[];
  inviteRoomId: string;
  // Room
  currentRoomId: string;
  currentSettings: RoomSettings;
  roomOwner: string;
  waitingPlayers: string[];
  // Game
  isPlaying: boolean;
  currentTurn: string;
  turnOrder: string[];
  currentWord: string;
  players: { name: string; score: number; lives: number }[];
  history: HistoryEntry[];
  timerSeconds: number;
  timerMax: number;
  maxLives: number;
  currentLives: Record<string, number>;
  lastWordPlayer: string;
  // Vote
  isVoteActive: boolean;
  vote: {
    voteType: 'challenge' | 'genre';
    word: string;
    player: string;
    challenger?: string;
    reason?: string;
    genre?: string;
    voteCount: number;
    totalPlayers: number;
    hasVoted: boolean;
    challengedPlayerName: string;
  } | null;
  // Game Over
  gameOver: {
    reason: string;
    winner?: string;
    loser?: string;
    scores: Record<string, number>;
    history: HistoryEntry[];
    lives: Record<string, number>;
    resultId?: string;
  } | null;
  // Messages
  messages: { text: string; type?: string; ts: string }[];
  toasts: Toast[];
  // Reconnect
  wasInRoom: string;
  wasRoomOwner: boolean;
  lastShareURL: string;
}

const defaultSettings: RoomSettings = {
  name: '',
  minLen: 2,
  maxLen: 20,
  genre: '',
  timeLimit: 30,
  maxLives: DEFAULT_MAX_LIVES,
};

const initialState: GameState = {
  screen: 'lobby',
  myName: '',
  // Lobby
  rooms: [],
  kanaRowNames: [],
  inviteRoomId: '',
  // Room
  currentRoomId: '',
  currentSettings: { ...defaultSettings },
  roomOwner: '',
  waitingPlayers: [],
  // Game
  isPlaying: false,
  currentTurn: '',
  turnOrder: [],
  currentWord: '',
  players: [],
  history: [],
  timerSeconds: 0,
  timerMax: 30,
  maxLives: DEFAULT_MAX_LIVES,
  currentLives: {},
  lastWordPlayer: '',
  // Vote
  isVoteActive: false,
  vote: null,
  // Game Over
  gameOver: null,
  // Messages
  messages: [],
  toasts: [],
  // Reconnect
  wasInRoom: '',
  wasRoomOwner: false,
  lastShareURL: '',
};

// Actions type
type Action =
  | { type: 'SET_NAME'; name: string }
  | { type: 'SET_ROOMS'; rooms: RoomInfo[] }
  | { type: 'SET_GENRES'; kanaRows: string[] }
  | { type: 'SET_INVITE_ROOM'; roomId: string }
  | { type: 'CLEAR_INVITE' }
  | { type: 'ROOM_JOINED'; msg: Extract<IncomingMessage, { type: 'room_joined' | 'room_state' }> }
  | { type: 'PLAYER_JOINED'; player: string }
  | { type: 'PLAYER_LEFT'; player: string }
  | { type: 'PLAYER_LIST'; players: string[] }
  | { type: 'GAME_STARTED'; msg: Extract<IncomingMessage, { type: 'game_started' }> }
  | { type: 'WORD_ACCEPTED'; msg: Extract<IncomingMessage, { type: 'word_accepted' }> }
  | { type: 'ANSWER_REJECTED'; message: string }
  | { type: 'TIMER'; timeLeft: number }
  | { type: 'GAME_OVER'; msg: Extract<IncomingMessage, { type: 'game_over' }> }
  | { type: 'VOTE_REQUEST'; msg: Extract<IncomingMessage, { type: 'vote_request' }> }
  | { type: 'VOTE_UPDATE'; msg: Extract<IncomingMessage, { type: 'vote_update' }> }
  | { type: 'VOTE_RESULT'; msg: Extract<IncomingMessage, { type: 'vote_result' }> }
  | { type: 'REBUTTAL'; msg: Extract<IncomingMessage, { type: 'rebuttal' }> }
  | { type: 'CHALLENGE_WITHDRAWN'; msg: Extract<IncomingMessage, { type: 'challenge_withdrawn' }> }
  | { type: 'PENALTY'; msg: Extract<IncomingMessage, { type: 'penalty' }> }
  | { type: 'TURN_UPDATE'; msg: Extract<IncomingMessage, { type: 'turn_update' }> }
  | { type: 'SETTINGS_UPDATED'; settings: RoomSettings }
  | { type: 'LEAVE_ROOM' }
  | { type: 'ADD_MESSAGE'; text: string; msgType?: string }
  | { type: 'ADD_TOAST'; toast: Toast }
  | { type: 'REMOVE_TOAST'; id: number }
  | { type: 'SET_SHARE_URL'; url: string }
  | { type: 'CLOSE_GAME_OVER' }
  | { type: 'REMEMBER_ROOM' };

export { type Action };

export function addTimestamp(): string {
  return new Date().toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function addMessage(state: GameState, text: string, msgType?: string): GameState {
  const ts = addTimestamp();
  const newMessages = [...state.messages, { text, type: msgType, ts }];
  // Keep last 100 messages
  if (newMessages.length > 100) {
    newMessages.splice(0, newMessages.length - 100);
  }
  return { ...state, messages: newMessages };
}

function buildPlayersFromMaps(
  turnOrder: string[],
  scores: Record<string, number>,
  lives: Record<string, number>
): { name: string; score: number; lives: number }[] {
  return turnOrder.map((name) => ({
    name,
    score: scores[name] ?? 0,
    lives: lives[name] ?? 0,
  }));
}

function reducer(state: GameState, action: Action): GameState {
  switch (action.type) {
    case 'SET_NAME':
      return { ...state, myName: action.name };

    case 'SET_ROOMS':
      return { ...state, rooms: action.rooms };

    case 'SET_GENRES':
      return { ...state, kanaRowNames: action.kanaRows };

    case 'SET_INVITE_ROOM':
      return { ...state, inviteRoomId: action.roomId };

    case 'CLEAR_INVITE':
      return { ...state, inviteRoomId: '' };

    case 'ROOM_JOINED': {
      const { msg } = action;
      const isPlaying = msg.status === 'playing';
      const players = buildPlayersFromMaps(
        msg.turnOrder.length > 0 ? msg.turnOrder : msg.players.map((p) => p.name),
        msg.scores,
        msg.lives
      );
      return {
        ...state,
        screen: 'game',
        currentRoomId: msg.roomId,
        roomOwner: msg.owner,
        currentSettings: msg.settings,
        waitingPlayers: msg.players.map((p) => p.name),
        isPlaying,
        currentTurn: msg.currentTurn,
        turnOrder: msg.turnOrder,
        currentWord: msg.currentWord,
        players,
        history: msg.history,
        maxLives: msg.maxLives || msg.settings.maxLives || DEFAULT_MAX_LIVES,
        currentLives: msg.lives,
        timerMax: msg.settings.timeLimit || 30,
        gameOver: null,
      };
    }

    case 'PLAYER_JOINED': {
      const wp = state.waitingPlayers.includes(action.player)
        ? state.waitingPlayers
        : [...state.waitingPlayers, action.player];
      const updated = { ...state, waitingPlayers: wp };
      return addMessage(updated, `${action.player} が入室しました`, 'info');
    }

    case 'PLAYER_LEFT': {
      const wp = state.waitingPlayers.filter((p) => p !== action.player);
      const updated = { ...state, waitingPlayers: wp };
      return addMessage(updated, `${action.player} が退室しました`, 'info');
    }

    case 'PLAYER_LIST':
      return { ...state, waitingPlayers: action.players };

    case 'GAME_STARTED': {
      const { msg } = action;
      const scores: Record<string, number> = {};
      for (const name of msg.turnOrder) {
        scores[name] = 0;
      }
      const players = buildPlayersFromMaps(msg.turnOrder, scores, msg.lives);
      const updated: GameState = {
        ...state,
        isPlaying: true,
        currentWord: msg.currentWord || msg.firstWord,
        turnOrder: msg.turnOrder,
        currentTurn: msg.currentTurn,
        players,
        history: [],
        maxLives: msg.maxLives || DEFAULT_MAX_LIVES,
        currentLives: msg.lives,
        timerSeconds: msg.timeLimit,
        timerMax: msg.timeLimit,
        lastWordPlayer: '',
        gameOver: null,
        isVoteActive: false,
        vote: null,
      };
      return addMessage(updated, `ゲーム開始！ 最初の文字: ${msg.currentWord || msg.firstWord}`, 'info');
    }

    case 'WORD_ACCEPTED': {
      const { msg } = action;
      const newHistory = [...state.history, { word: msg.word, player: msg.player }];
      const players = buildPlayersFromMaps(state.turnOrder, msg.scores, msg.lives);
      return {
        ...state,
        currentWord: msg.word,
        history: newHistory,
        players,
        currentLives: msg.lives,
        currentTurn: msg.currentTurn,
        lastWordPlayer: msg.player,
      };
    }

    case 'ANSWER_REJECTED': {
      return addMessage(state, action.message, 'error');
    }

    case 'TIMER':
      return { ...state, timerSeconds: action.timeLeft };

    case 'GAME_OVER': {
      const { msg } = action;
      const updated: GameState = {
        ...state,
        isPlaying: false,
        gameOver: {
          reason: msg.reason,
          winner: msg.winner,
          loser: msg.loser,
          scores: msg.scores,
          history: msg.history,
          lives: msg.lives,
          resultId: msg.resultId,
        },
        isVoteActive: false,
        vote: null,
      };
      return addMessage(updated, `ゲーム終了: ${msg.reason}`, 'info');
    }

    case 'VOTE_REQUEST': {
      const { msg } = action;
      return {
        ...state,
        isVoteActive: true,
        vote: {
          voteType: msg.voteType,
          word: msg.word,
          player: msg.player,
          challenger: msg.challenger,
          reason: msg.reason,
          genre: msg.genre,
          voteCount: msg.voteCount,
          totalPlayers: msg.totalPlayers,
          hasVoted: false,
          challengedPlayerName: msg.player,
        },
      };
    }

    case 'VOTE_UPDATE': {
      const { msg } = action;
      if (!state.vote) return state;
      return {
        ...state,
        vote: {
          ...state.vote,
          voteCount: msg.voteCount,
          totalPlayers: msg.totalPlayers,
        },
      };
    }

    case 'VOTE_RESULT': {
      const { msg } = action;
      let updated: GameState = {
        ...state,
        isVoteActive: false,
        vote: null,
      };
      // Apply reverted state if provided
      if (msg.history !== undefined) {
        updated = { ...updated, history: msg.history };
      }
      if (msg.currentWord !== undefined) {
        updated = { ...updated, currentWord: msg.currentWord };
      }
      if (msg.scores !== undefined) {
        const lives = msg.lives ?? state.currentLives;
        updated = {
          ...updated,
          players: buildPlayersFromMaps(state.turnOrder, msg.scores, lives),
        };
      }
      if (msg.lives !== undefined) {
        updated = { ...updated, currentLives: msg.lives };
        // Rebuild players with updated lives
        const scores = msg.scores ?? Object.fromEntries(state.players.map((p) => [p.name, p.score]));
        updated = {
          ...updated,
          players: buildPlayersFromMaps(state.turnOrder, scores, msg.lives),
        };
      }
      if (msg.currentTurn !== undefined) {
        updated = { ...updated, currentTurn: msg.currentTurn };
      }
      const resultMsg = msg.accepted
        ? `チャレンジ成功: ${msg.word} - ${msg.message ?? ''}`
        : `チャレンジ失敗: ${msg.word} - ${msg.message ?? ''}`;
      return addMessage(updated, resultMsg, 'info');
    }

    case 'REBUTTAL': {
      const { msg } = action;
      return addMessage(state, `${msg.player} の反論: ${msg.rebuttal}`, 'info');
    }

    case 'CHALLENGE_WITHDRAWN': {
      const { msg } = action;
      const updated: GameState = {
        ...state,
        isVoteActive: false,
        vote: null,
      };
      return addMessage(updated, msg.message ?? 'チャレンジが取り下げられました', 'info');
    }

    case 'PENALTY': {
      const { msg } = action;
      const newLives = { ...state.currentLives, ...msg.allLives };
      const scores = Object.fromEntries(state.players.map((p) => [p.name, p.score]));
      const players = buildPlayersFromMaps(state.turnOrder, scores, newLives);
      const updated: GameState = {
        ...state,
        currentLives: newLives,
        players,
      };
      const elimMsg = msg.eliminated ? `（脱落！）` : '';
      return addMessage(updated, `${msg.player} にペナルティ: ${msg.reason} (残りライフ: ${msg.lives})${elimMsg}`, 'info');
    }

    case 'TURN_UPDATE': {
      const { msg } = action;
      const players = buildPlayersFromMaps(msg.turnOrder, msg.scores, msg.lives);
      return {
        ...state,
        turnOrder: msg.turnOrder,
        currentTurn: msg.currentTurn,
        players,
        currentLives: msg.lives,
        maxLives: msg.maxLives || state.maxLives,
      };
    }

    case 'SETTINGS_UPDATED':
      return { ...state, currentSettings: action.settings };

    case 'LEAVE_ROOM':
      return {
        ...state,
        screen: 'lobby',
        currentRoomId: '',
        roomOwner: '',
        currentSettings: { ...defaultSettings },
        waitingPlayers: [],
        isPlaying: false,
        currentTurn: '',
        turnOrder: [],
        currentWord: '',
        players: [],
        history: [],
        timerSeconds: 0,
        maxLives: DEFAULT_MAX_LIVES,
        currentLives: {},
        lastWordPlayer: '',
        isVoteActive: false,
        vote: null,
        gameOver: null,
      };

    case 'ADD_MESSAGE':
      return addMessage(state, action.text, action.msgType);

    case 'ADD_TOAST':
      return { ...state, toasts: [...state.toasts, action.toast] };

    case 'REMOVE_TOAST':
      return { ...state, toasts: state.toasts.filter((t) => t.id !== action.id) };

    case 'SET_SHARE_URL':
      return { ...state, lastShareURL: action.url };

    case 'CLOSE_GAME_OVER':
      return { ...state, gameOver: null };

    case 'REMEMBER_ROOM':
      return {
        ...state,
        wasInRoom: state.currentRoomId,
        wasRoomOwner: state.roomOwner === state.myName,
      };

    default:
      return state;
  }
}

export function useGameState() {
  return useReducer(reducer, initialState);
}
