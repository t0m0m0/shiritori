import { useState, useCallback } from 'react';
import type { RoomInfo, OutgoingMessage } from '../../types/messages';
import type { GameState, Action } from '../../hooks/useGameState';
import { CreateRoom } from './CreateRoom';
import { RoomList } from './RoomList';
import { InviteCard } from './InviteCard';

interface Props {
  state: GameState;
  dispatch: React.Dispatch<Action>;
  onSend: (msg: OutgoingMessage) => void;
}

export function Lobby({ state, dispatch, onSend }: Props) {
  const [playerName, setPlayerName] = useState(state.myName);

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPlayerName(e.target.value);
    dispatch({ type: 'SET_NAME', name: e.target.value });
  };

  const handleJoinRoom = useCallback((roomId: string) => {
    const name = playerName.trim();
    if (!name) return;
    dispatch({ type: 'SET_NAME', name });
    onSend({ type: 'join', name, roomId });
    // Update URL
    const url = new URL(window.location.href);
    url.searchParams.set('room', roomId);
    window.history.replaceState({}, '', url.toString());
  }, [playerName, dispatch, onSend]);

  const handleRefresh = useCallback(() => {
    onSend({ type: 'get_rooms' });
  }, [onSend]);

  const handleJoinInvite = useCallback(() => {
    if (!state.inviteRoomId) return;
    handleJoinRoom(state.inviteRoomId);
  }, [state.inviteRoomId, handleJoinRoom]);

  const handleClearInvite = useCallback(() => {
    dispatch({ type: 'CLEAR_INVITE' });
    const url = new URL(window.location.href);
    url.searchParams.delete('room');
    window.history.replaceState({}, '', url.toString());
  }, [dispatch]);

  return (
    <div className="container fade-in">
      <div className="player-name-section">
        <div className="player-name-desc"><p>名前を入力して、ルームに参加するか、新しいルームを作成してください。</p></div>
        <div className="player-name-field">
          <label htmlFor="playerName" className="player-name-label">ユーザー名</label>
          <input type="text" id="playerName" placeholder="名前を入力" maxLength={12} autoComplete="off"
            value={playerName} onChange={handleNameChange} />
        </div>
      </div>

      <CreateRoom playerName={playerName} kanaRowNames={state.kanaRowNames} onSend={onSend} />

      <InviteCard inviteRoomId={state.inviteRoomId} playerName={playerName}
        onJoin={handleJoinInvite} onClear={handleClearInvite} />

      <RoomList rooms={state.rooms} playerName={playerName}
        onJoinRoom={handleJoinRoom} onRefresh={handleRefresh} />
    </div>
  );
}
