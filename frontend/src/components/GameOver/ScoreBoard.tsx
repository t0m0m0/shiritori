import { useState, useCallback, useMemo } from 'react';
import type { RoomSettings, HistoryEntry, OutgoingMessage } from '../../types/messages';

const DEFAULT_MAX_LIVES = 3;

interface GameOverData {
  reason: string;
  winner?: string;
  loser?: string;
  scores: Record<string, number>;
  history: HistoryEntry[];
  lives: Record<string, number>;
  resultId?: string;
}

interface Props {
  gameOver: GameOverData;
  currentSettings: RoomSettings;
  myName: string;
  roomOwner: string;
  kanaRowNames: string[];
  onSend: (msg: OutgoingMessage) => void;
  onBackToLobby: () => void;
  lastShareURL: string;
}

export function ScoreBoard({ gameOver, currentSettings, myName, roomOwner, kanaRowNames, onSend, onBackToLobby, lastShareURL }: Props) {
  const [historyOpen, setHistoryOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [copiedLink, setCopiedLink] = useState(false);

  // Settings form state
  const [minLen, setMinLen] = useState(currentSettings.minLen || 1);
  const [maxLen, setMaxLen] = useState(currentSettings.maxLen || 0);
  const [genre, setGenre] = useState(currentSettings.genre || '');
  const [timeLimit, setTimeLimit] = useState(currentSettings.timeLimit || 0);
  const [maxLives, setMaxLives] = useState(currentSettings.maxLives || DEFAULT_MAX_LIVES);
  const [selectedRows, setSelectedRows] = useState<string[]>(currentSettings.allowedRows || []);
  const [noDakuten, setNoDakuten] = useState(!!currentSettings.noDakuten);
  const [waitingForHost, setWaitingForHost] = useState(false);

  const isOwner = myName === roomOwner;

  const sorted = useMemo(() =>
    Object.entries(gameOver.scores).sort((a, b) => b[1] - a[1]),
    [gameOver.scores]
  );
  const medals = ['🥇', '🥈', '🥉'];

  let reason = gameOver.reason || '';
  if (gameOver.winner) {
    reason = `🏆 ${gameOver.winner}さんの勝利！${gameOver.loser ? ` (${gameOver.loser}さん脱落)` : ''}`;
  } else if (gameOver.loser) {
    reason = `${gameOver.loser}さん - ${reason}`;
  }

  const chain = gameOver.history.map((h) => h.word).join(' → ');

  const settingsChanged = useMemo(() => {
    const s = currentSettings;
    return (
      minLen !== (s.minLen || 1) ||
      maxLen !== (s.maxLen || 0) ||
      genre !== (s.genre || '') ||
      timeLimit !== (s.timeLimit || 0) ||
      maxLives !== (s.maxLives || DEFAULT_MAX_LIVES) ||
      noDakuten !== !!s.noDakuten ||
      JSON.stringify(selectedRows.length > 0 ? selectedRows : []) !== JSON.stringify(s.allowedRows || [])
    );
  }, [minLen, maxLen, genre, timeLimit, maxLives, noDakuten, selectedRows, currentSettings]);

  const handlePlayAgain = useCallback(() => {
    if (!isOwner) {
      setWaitingForHost(true);
      return;
    }
    if (settingsChanged) {
      const newSettings: RoomSettings = {
        name: currentSettings.name || 'しりとりルーム',
        minLen, maxLen, genre, timeLimit, maxLives,
        allowedRows: selectedRows.length > 0 ? selectedRows : undefined,
        noDakuten: noDakuten || undefined,
        private: currentSettings.private || undefined,
      };
      onSend({ type: 'start_game', settings: newSettings });
    } else {
      onSend({ type: 'start_game' });
    }
  }, [isOwner, settingsChanged, minLen, maxLen, genre, timeLimit, maxLives, selectedRows, noDakuten, currentSettings, onSend]);

  const shareURL = lastShareURL || (gameOver.resultId ? `${location.origin}/results/${gameOver.resultId}` : '');

  const handleShareX = useCallback(() => {
    if (!shareURL) return;
    const words = gameOver.history.map((h) => h.word);
    const chainStr = words.join(' → ');
    let text = `しりとりで${words.length}語つなぎました！\n`;
    if ([...chainStr].length > 140) {
      text += [...chainStr].slice(0, 137).join('') + '…\n';
    } else {
      text += chainStr + '\n';
    }
    window.open(`https://x.com/intent/tweet?text=${encodeURIComponent(text)}&url=${encodeURIComponent(shareURL)}`, '_blank', 'width=550,height=420');
  }, [shareURL, gameOver.history]);

  const handleShareLINE = useCallback(() => {
    if (!shareURL) return;
    const words = gameOver.history.map((h) => h.word);
    const chainStr = words.join(' → ');
    let text = `しりとりで${words.length}語つなぎました！\n`;
    if ([...chainStr].length > 200) {
      text += [...chainStr].slice(0, 197).join('') + '…\n';
    } else {
      text += chainStr + '\n';
    }
    text += shareURL;
    window.open(`https://social-plugins.line.me/lineit/share?url=${encodeURIComponent(shareURL)}&text=${encodeURIComponent(text)}`, '_blank', 'width=550,height=420');
  }, [shareURL, gameOver.history]);

  const handleCopyLink = useCallback(async () => {
    if (!shareURL) return;
    try {
      await navigator.clipboard.writeText(shareURL);
      setCopiedLink(true);
      setTimeout(() => setCopiedLink(false), 2000);
    } catch {
      prompt('リンクをコピー:', shareURL);
    }
  }, [shareURL]);

  return (
    <div className="game-over-overlay">
      <div className="game-over-card">
        <h2>ゲーム終了！</h2>
        <p className="game-over-reason">{reason}</p>

        <ul className="final-scores">
          {sorted.map(([name, score], i) => (
            <li key={name} className="final-score-item">
              <span className="final-rank">{medals[i] || i + 1}</span>
              <span className="final-name">{name}</span>
              <span className="final-pts">{score}点</span>
            </li>
          ))}
        </ul>

        {/* History */}
        <div className="game-over-history">
          <button className={`game-over-history-toggle${historyOpen ? ' open' : ''}`}
            onClick={() => setHistoryOpen(!historyOpen)}>
            <span>📜 履歴を見る（{gameOver.history.length}語）</span>
            <span className="toggle-arrow">▼</span>
          </button>
          <div className={`game-over-history-body${historyOpen ? ' open' : ''}`}>
            {gameOver.history.length > 0 && (
              <div className="game-over-history-chain">{chain}</div>
            )}
            <ul className="game-over-history-list">
              {gameOver.history.map((h, i) => (
                <li key={i}>
                  <span className="game-over-history-num">{i + 1}.</span>
                  <span className="game-over-history-word">{h.word}</span>
                  <span className="game-over-history-player">{h.player}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Share */}
        {shareURL && (
          <div className="share-section">
            <p>📢 結果をシェア</p>
            <div className="share-buttons">
              <button className="share-btn share-btn-x" onClick={handleShareX}>
                <span className="share-icon">𝕏</span> ポスト
              </button>
              <button className="share-btn share-btn-line" onClick={handleShareLINE}>
                <span className="share-icon">💬</span> LINE
              </button>
              <button className={`share-btn share-btn-copy${copiedLink ? ' copied' : ''}`} onClick={handleCopyLink}>
                <span className="share-icon">{copiedLink ? '✔' : '🔗'}</span> {copiedLink ? 'コピーしました' : 'リンクコピー'}
              </button>
            </div>
          </div>
        )}

        {/* Settings (owner only) */}
        {isOwner && (
          <div className="game-over-settings">
            <button className={`game-over-settings-toggle${settingsOpen ? ' open' : ''}`}
              onClick={() => setSettingsOpen(!settingsOpen)}>
              <span>⚙️ ルール変更 {settingsChanged && <span className="game-over-settings-changed visible">✏️ 変更あり</span>}</span>
              <span className="toggle-arrow">▼</span>
            </button>
            <div className={`game-over-settings-body${settingsOpen ? ' open' : ''}`}>
              <div className="form-row">
                <div className="form-group">
                  <label>最少文字数</label>
                  <input type="number" value={minLen} min={1} max={20} onChange={(e) => setMinLen(Number(e.target.value))} />
                </div>
                <div className="form-group">
                  <label>最大文字数（0＝制限なし）</label>
                  <input type="number" value={maxLen} min={0} max={99} onChange={(e) => setMaxLen(Number(e.target.value))} />
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label>ジャンル</label>
                  <input type="text" placeholder="例: 食べ物、動物…" maxLength={20} value={genre} onChange={(e) => setGenre(e.target.value)} />
                </div>
                <div className="form-group">
                  <label>制限時間</label>
                  <select value={timeLimit} onChange={(e) => setTimeLimit(Number(e.target.value))}>
                    <option value={0}>なし</option>
                    <option value={10}>10秒</option>
                    <option value={20}>20秒</option>
                    <option value={30}>30秒</option>
                    <option value={60}>60秒</option>
                  </select>
                </div>
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label>ライフ数</label>
                  <select value={maxLives} onChange={(e) => setMaxLives(Number(e.target.value))}>
                    <option value={1}>❤️</option>
                    <option value={2}>❤️❤️</option>
                    <option value={3}>❤️❤️❤️</option>
                    <option value={5}>❤️❤️❤️❤️❤️</option>
                    <option value={10}>❤️×10</option>
                  </select>
                </div>
                <div className="form-group"></div>
              </div>
              <div className="form-group">
                <label>使用可能な行（未選択＝すべて）</label>
                <div className="kana-row-grid">
                  {kanaRowNames.map((name) => (
                    <label key={name} className={`kana-row-chip${selectedRows.includes(name) ? ' selected' : ''}`}
                      onClick={() => setSelectedRows((prev) => prev.includes(name) ? prev.filter((r) => r !== name) : [...prev, name])}>
                      <input type="checkbox" checked={selectedRows.includes(name)} readOnly /> {name}
                    </label>
                  ))}
                </div>
              </div>
              <div className="form-group">
                <label className="kana-row-chip" style={{ display: 'inline-flex', cursor: 'pointer' }}>
                  <input type="checkbox" checked={noDakuten} onChange={(e) => setNoDakuten(e.target.checked)}
                    style={{ display: 'inline', width: 'auto', marginRight: '0.3rem' }} />
                  濁音・半濁音禁止
                </label>
              </div>
            </div>
          </div>
        )}

        {!waitingForHost ? (
          <>
            <button className="btn btn-primary btn-lg" onClick={handlePlayAgain} style={{ marginRight: '0.5rem' }}>
              {settingsChanged ? '🔄 ルール変更して開始' : '🔄 もう一度'}
            </button>
            <button className="btn btn-outline btn-lg" onClick={onBackToLobby}>🏠 ロビーへ</button>
          </>
        ) : (
          <p className="game-over-reason">ホストの開始を待っています…</p>
        )}
      </div>
    </div>
  );
}
