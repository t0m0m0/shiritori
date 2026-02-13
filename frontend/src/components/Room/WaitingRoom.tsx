import type { OutgoingMessage } from '../../types/messages';

interface Props {
  waitingPlayers: string[];
  roomOwner: string;
  myName: string;
  onSend: (msg: OutgoingMessage) => void;
}

export function WaitingRoom({ waitingPlayers, roomOwner, myName, onSend }: Props) {
  const isOwner = myName === roomOwner;

  return (
    <div className="card start-area">
      <p className="waiting-text">プレイヤーを待っています…</p>
      <div className="waiting-players">
        <h3>👥 参加者</h3>
        <ul className="waiting-player-list">
          {waitingPlayers.length === 0 ? (
            <li className="waiting-empty">参加者なし</li>
          ) : (
            waitingPlayers.map((name) => (
              <li key={name}>
                {name}
                {name === roomOwner && <span className="owner-badge">ホスト</span>}
                {name === myName && ' 👈'}
              </li>
            ))
          )}
        </ul>
      </div>
      {isOwner ? (
        <button className="btn btn-accent btn-lg" onClick={() => onSend({ type: 'start_game' })}>
          🎮 ゲーム開始
        </button>
      ) : (
        <p className="waiting-text">ルーム作成者の開始を待っています…</p>
      )}
    </div>
  );
}
