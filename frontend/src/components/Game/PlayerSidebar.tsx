interface PlayerData {
  name: string;
  score: number;
  lives: number;
}

interface Props {
  players: PlayerData[];
  myName: string;
  currentTurn: string;
  maxLives: number;
  messages: { text: string; type?: string; ts: string }[];
}

export function PlayerSidebar({ players, myName, currentTurn, maxLives, messages }: Props) {
  const sorted = [...players].sort((a, b) => b.score - a.score);

  return (
    <div className="sidebar">
      <div className="card">
        <h2>プレイヤー</h2>
        <ul className="player-list">
          {sorted.map((p) => {
            const heartsStr = p.lives > 0 ? '❤️'.repeat(Math.min(p.lives, 10)) : '💀';
            return (
              <li key={p.name} className={`player-item${p.name === currentTurn ? ' current-turn' : ''}`}
                style={p.lives <= 0 ? { opacity: 0.4 } : undefined}>
                <span className="player-name-display">
                  {p.name}{p.name === myName && ' 👈'}
                </span>
                <span className="player-lives">{heartsStr}</span>
                <span className="player-score">{p.score}点</span>
              </li>
            );
          })}
        </ul>
      </div>
      <div className="card">
        <h2>メッセージ</h2>
        <ul className="messages">
          {messages.map((m, i) => (
            <li key={i} className={`msg-item${m.type ? ` msg-${m.type}` : ''}`}>
              [{m.ts}] {m.text}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
