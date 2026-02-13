import type { HistoryEntry } from '../../types/messages';

interface Props {
  history: HistoryEntry[];
}

export function WordHistory({ history }: Props) {
  return (
    <div className="history-panel card">
      <h2>履歴</h2>
      <ul className="history-list">
        {[...history].reverse().map((h, i) => (
          <li key={history.length - 1 - i} className="history-item">
            <span className="history-word">{h.word}</span>
            <span className="history-player">{h.player}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
