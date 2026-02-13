import type { RoomInfo, OutgoingMessage } from '../../types/messages';
import { getRoomLink } from '../../utils/helpers';

interface Props {
  rooms: RoomInfo[];
  playerName: string;
  onJoinRoom: (roomId: string) => void;
  onRefresh: () => void;
}

export function RoomList({ rooms, playerName, onJoinRoom, onRefresh }: Props) {
  const hasName = playerName.trim().length > 0;

  return (
    <div className="card slide-up" style={{ animationDelay: '0.1s' }}>
      <h2>
        ルーム一覧
        <button className="btn btn-outline" style={{ marginLeft: 'auto', padding: '0.3rem 0.8rem', fontSize: '0.8rem' }}
          onClick={onRefresh}>更新</button>
      </h2>
      {rooms.length === 0 ? (
        <div className="no-rooms">現在アクティブなルームはありません</div>
      ) : (
        <ul className="room-list">
          {rooms.map((r) => {
            const isPlaying = r.status === 'playing';
            const genreLabel = r.settings?.genre || 'なし';
            const playerCount = r.playerCount ?? r.players ?? 0;
            const statusLabel = isPlaying ? '🎮 プレイ中' : '⏳ 待機中';
            return (
              <li key={r.id} className="room-item fade-in">
                <div className="room-info">
                  <a className="room-name" href={getRoomLink(r.id)}
                    onClick={(e) => { e.preventDefault(); onJoinRoom(r.id); }}>
                    {r.name}
                  </a>
                  <div className="room-meta">
                    <span>👥 {playerCount}人</span>
                    <span>🏷️ {genreLabel}</span>
                    <span>{statusLabel}</span>
                  </div>
                </div>
                <div className="room-actions">
                  <div className="lobby-btn-wrap">
                    <button className="btn btn-primary"
                      onClick={() => onJoinRoom(r.id)}
                      disabled={isPlaying || !hasName}>
                      参加
                    </button>
                    {!hasName && !isPlaying && <span className="lobby-btn-tooltip">ユーザー名を入力してください</span>}
                    {isPlaying && <span className="lobby-btn-tooltip">プレイ中です</span>}
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
