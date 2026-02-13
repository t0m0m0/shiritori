import type { RoomSettings } from '../../types/messages';
import { RuleBadges } from '../common/RuleBadges';
import { useEffect, useState } from 'react';

interface InviteRoomData {
  id: string;
  name: string;
  owner: string;
  playerCount: number;
  settings: RoomSettings;
  players: string[];
}

interface Props {
  inviteRoomId: string;
  playerName: string;
  onJoin: () => void;
  onClear: () => void;
}

export function InviteCard({ inviteRoomId, playerName, onJoin, onClear }: Props) {
  const [roomData, setRoomData] = useState<InviteRoomData | null>(null);
  const hasName = playerName.trim().length > 0;

  useEffect(() => {
    if (!inviteRoomId) {
      setRoomData(null);
      return;
    }
    fetch(`/room/${encodeURIComponent(inviteRoomId)}`)
      .then((res) => res.ok ? res.json() : null)
      .then((data) => {
        if (data) setRoomData(data);
        else onClear();
      })
      .catch(() => onClear());
  }, [inviteRoomId, onClear]);

  if (!inviteRoomId || !roomData) return null;

  return (
    <div className="card invite-card slide-up" style={{ animationDelay: '0.05s' }}>
      <div className="invite-info">
        <div className="invite-room-title">{roomData.name || '招待ルーム'}</div>
        <div className="invite-room-rules">
          <RuleBadges settings={roomData.settings} showPrivate owner={roomData.owner} playerCount={roomData.playerCount} />
        </div>
        <div className="invite-room-host">{roomData.owner ? `ホスト: ${roomData.owner}` : ''}</div>
      </div>
      <div className="invite-actions">
        <div className="lobby-btn-wrap">
          <button className="btn btn-primary" onClick={onJoin} disabled={!hasName}>参加する</button>
          {!hasName && <span className="lobby-btn-tooltip">ユーザー名を入力してください</span>}
        </div>
        <button className="btn btn-outline" onClick={onClear}>無視</button>
      </div>
    </div>
  );
}
