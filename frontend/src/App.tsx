import { useCallback, useEffect, useRef, useState } from 'react';
import { useWebSocket } from './hooks/useWebSocket';
import { useGameState } from './hooks/useGameState';
import type { IncomingMessage, OutgoingMessage } from './types/messages';
import { nextToastId } from './utils/helpers';
import { ToastContainer } from './components/common/Toast';
import { ThemeSwitcher } from './components/common/ThemeSwitcher';
import { Lobby } from './components/Lobby';
import { GameRoom } from './components/Game';
import { VotePanel } from './components/Vote/VotePanel';
import { ScoreBoard } from './components/GameOver/ScoreBoard';
import './styles/game.css';

function App() {
  const [state, dispatch] = useGameState();
  const [rebuttals, setRebuttals] = useState<{ player: string; text: string }[]>([]);
  const initializedRef = useRef(false);

  const handleMessage = useCallback(
    (msg: IncomingMessage) => {
      switch (msg.type) {
        case 'rooms':
          dispatch({ type: 'SET_ROOMS', rooms: msg.rooms || [] });
          break;
        case 'genres':
          dispatch({ type: 'SET_GENRES', kanaRows: msg.kanaRows || [] });
          break;
        case 'room_joined':
        case 'room_state':
          dispatch({ type: 'ROOM_JOINED', msg });
          dispatch({ type: 'ADD_MESSAGE', text: 'ルームに参加しました', msgType: 'info' });
          break;
        case 'player_joined':
          dispatch({ type: 'PLAYER_JOINED', player: msg.player });
          break;
        case 'player_left':
          dispatch({ type: 'PLAYER_LEFT', player: msg.player });
          break;
        case 'player_list':
          dispatch({ type: 'PLAYER_LIST', players: msg.players });
          break;
        case 'game_started':
          dispatch({ type: 'GAME_STARTED', msg });
          break;
        case 'word_accepted':
          dispatch({ type: 'WORD_ACCEPTED', msg });
          dispatch({
            type: 'ADD_MESSAGE',
            text: `${msg.player}さんが正解！「${msg.word}」`,
            msgType: 'success',
          });
          break;
        case 'answer_rejected':
          dispatch({ type: 'ANSWER_REJECTED', message: msg.message });
          break;
        case 'timer':
          dispatch({ type: 'TIMER', timeLeft: msg.timeLeft });
          break;
        case 'game_over':
          dispatch({ type: 'GAME_OVER', msg });
          if (msg.resultId) {
            dispatch({ type: 'SET_SHARE_URL', url: `${location.origin}/results/${msg.resultId}` });
          }
          break;
        case 'vote_request':
          dispatch({ type: 'VOTE_REQUEST', msg });
          setRebuttals([]);
          break;
        case 'vote_update':
          dispatch({ type: 'VOTE_UPDATE', msg });
          break;
        case 'vote_result':
          dispatch({ type: 'VOTE_RESULT', msg });
          setRebuttals([]);
          break;
        case 'rebuttal':
          dispatch({ type: 'REBUTTAL', msg });
          setRebuttals((prev) => [...prev, { player: msg.player, text: msg.rebuttal }]);
          break;
        case 'challenge_withdrawn':
          dispatch({ type: 'CHALLENGE_WITHDRAWN', msg });
          setRebuttals([]);
          break;
        case 'penalty':
          dispatch({ type: 'PENALTY', msg });
          break;
        case 'turn_update':
          dispatch({ type: 'TURN_UPDATE', msg });
          break;
        case 'settings_updated':
          dispatch({ type: 'SETTINGS_UPDATED', settings: msg.settings });
          dispatch({ type: 'ADD_MESSAGE', text: '⚙️ ルールが変更されました', msgType: 'info' });
          break;
        case 'error':
          dispatch({ type: 'ADD_MESSAGE', text: msg.message, msgType: 'error' });
          dispatch({
            type: 'ADD_TOAST',
            toast: { id: nextToastId(), message: msg.message, type: 'error' },
          });
          break;
        default:
          console.log('Unknown message', msg);
      }
    },
    [dispatch],
  );

  const { send } = useWebSocket(handleMessage);

  // On WS connect: refresh rooms and get genres
  const wsInitRef = useRef(false);
  useEffect(() => {
    // Small delay to ensure WS is ready
    const timer = setTimeout(() => {
      if (!wsInitRef.current) {
        wsInitRef.current = true;
        send({ type: 'get_rooms' });
        send({ type: 'get_genres' });
      }
    }, 500);
    return () => clearTimeout(timer);
  }, [send]);

  // Handle invite from URL
  useEffect(() => {
    if (initializedRef.current) return;
    initializedRef.current = true;
    const params = new URLSearchParams(window.location.search);
    const roomId = params.get('room');
    if (roomId) {
      dispatch({ type: 'SET_INVITE_ROOM', roomId });
    }
  }, [dispatch]);

  // Periodic room refresh
  useEffect(() => {
    const interval = setInterval(() => {
      if (state.screen === 'lobby') {
        send({ type: 'get_rooms' });
      }
    }, 5000);
    return () => clearInterval(interval);
  }, [state.screen, send]);

  const handleSend = useCallback(
    (msg: OutgoingMessage) => {
      // Set name on create/join
      if (msg.type === 'create_room' || msg.type === 'join') {
        dispatch({ type: 'SET_NAME', name: msg.name });
      }
      send(msg);
    },
    [send, dispatch],
  );

  const handleRemoveToast = useCallback(
    (id: number) => {
      dispatch({ type: 'REMOVE_TOAST', id });
    },
    [dispatch],
  );

  const handleBackToLobby = useCallback(() => {
    send({ type: 'leave_room' });
    dispatch({ type: 'LEAVE_ROOM' });
    send({ type: 'get_rooms' });
  }, [send, dispatch]);

  return (
    <>
      <ToastContainer toasts={state.toasts} onRemove={handleRemoveToast} />

      <div className="header">
        <h1>
          <a href="/" onClick={(e) => { if (state.screen === 'lobby') { e.preventDefault(); } }}>
            しりとり
          </a>
        </h1>
        <p>ことばを繋ぐ、みんなで遊ぶ</p>
      </div>

      <ThemeSwitcher />

      {state.screen === 'lobby' && (
        <Lobby state={state} dispatch={dispatch} onSend={handleSend} />
      )}

      {state.screen === 'game' && (
        <GameRoom state={state} dispatch={dispatch} onSend={handleSend} />
      )}

      {state.isVoteActive && state.vote && (
        <VotePanel
          vote={state.vote}
          myName={state.myName}
          onSend={handleSend}
          rebuttals={rebuttals}
        />
      )}

      {state.gameOver && (
        <ScoreBoard
          gameOver={state.gameOver}
          currentSettings={state.currentSettings}
          myName={state.myName}
          roomOwner={state.roomOwner}
          kanaRowNames={state.kanaRowNames}
          onSend={handleSend}
          onBackToLobby={handleBackToLobby}
          lastShareURL={state.lastShareURL}
        />
      )}
    </>
  );
}

export default App;
