import { useState, useCallback } from 'react';
import type { RoomSettings, OutgoingMessage } from '../../types/messages';

const DEFAULT_MAX_LIVES = 3;

interface Props {
  playerName: string;
  kanaRowNames: string[];
  onSend: (msg: OutgoingMessage) => void;
}

export function CreateRoom({ playerName, kanaRowNames, onSend }: Props) {
  const [roomName, setRoomName] = useState('');
  const [minLen, setMinLen] = useState(1);
  const [maxLen, setMaxLen] = useState(0);
  const [genre, setGenre] = useState('');
  const [timeLimit, setTimeLimit] = useState(30);
  const [maxLives, setMaxLives] = useState(DEFAULT_MAX_LIVES);
  const [selectedRows, setSelectedRows] = useState<string[]>([]);
  const [noDakuten, setNoDakuten] = useState(false);
  const [isPrivate, setIsPrivate] = useState(false);

  const hasName = playerName.trim().length > 0;

  const toggleRow = useCallback((row: string) => {
    setSelectedRows((prev) =>
      prev.includes(row) ? prev.filter((r) => r !== row) : [...prev, row]
    );
  }, []);

  const handleCreate = () => {
    if (!hasName) return;
    const settings: RoomSettings = {
      name: roomName.trim() || 'しりとりルーム',
      minLen: minLen || 1,
      maxLen: maxLen || 0,
      genre,
      timeLimit: timeLimit || 0,
      maxLives: maxLives || DEFAULT_MAX_LIVES,
      allowedRows: selectedRows.length > 0 ? selectedRows : undefined,
      noDakuten: noDakuten || undefined,
      private: isPrivate || undefined,
    };
    onSend({ type: 'create_room', name: playerName.trim(), settings });
  };

  return (
    <div className="card slide-up">
      <h2>ルームを作る</h2>
      <div className="create-room-layout">
        <div>
          <div className="form-group">
            <label>ルーム名</label>
            <input type="text" placeholder="楽しいしりとり" maxLength={20} value={roomName} onChange={(e) => setRoomName(e.target.value)} />
          </div>
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
              <label>ジャンル（自由入力）</label>
              <input type="text" placeholder="例: 食べ物、動物、国名..." maxLength={20} value={genre} onChange={(e) => setGenre(e.target.value)} />
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
            <label>使用可能な行（未選択＝すべて使用可能）</label>
            <div className="kana-row-grid">
              {kanaRowNames.map((name) => (
                <label key={name} className={`kana-row-chip${selectedRows.includes(name) ? ' selected' : ''}`}
                  onClick={() => toggleRow(name)}>
                  <input type="checkbox" checked={selectedRows.includes(name)} readOnly /> {name}
                </label>
              ))}
            </div>
          </div>
          <div className="form-group">
            <label className="kana-row-chip" style={{ display: 'inline-flex', cursor: 'pointer' }}>
              <input type="checkbox" checked={noDakuten} onChange={(e) => setNoDakuten(e.target.checked)}
                style={{ display: 'inline', width: 'auto', marginRight: '0.3rem' }} />
              濁音・半濁音禁止（がぎぐげござじずぜぞだぢづでどばびぶべぼぱぴぷぺぽ）
            </label>
          </div>
          <div className="form-group">
            <label className="kana-row-chip" style={{ display: 'inline-flex', cursor: 'pointer' }}>
              <input type="checkbox" checked={isPrivate} onChange={(e) => setIsPrivate(e.target.checked)}
                style={{ display: 'inline', width: 'auto', marginRight: '0.3rem' }} />
              🔒 プライベートルーム（ロビーに表示しない）
            </label>
          </div>
          <div className="lobby-btn-wrap" style={{ display: 'block' }}>
            <button className="btn btn-primary btn-block" onClick={handleCreate} disabled={!hasName}>
              ルームを作成
            </button>
            {!hasName && <span className="lobby-btn-tooltip">ユーザー名を入力してください</span>}
          </div>
        </div>
        <div className="rules-panel">
          <table className="rules-table">
            <tbody>
              <tr><td>「ん」で終了</td><td className="rules-val">ライフ −1</td></tr>
              <tr><td>同じ単語</td><td className="rules-val">ライフ −1</td></tr>
              <tr><td>制限時間超過</td><td className="rules-val">ライフ −1</td></tr>
              <tr><td>ライフ 0</td><td className="rules-val rules-danger">敗北</td></tr>
            </tbody>
          </table>
          <p className="rules-desc">最後まで生き残ったプレイヤーの勝利。言葉の知識と反射神経が試される。</p>
        </div>
      </div>
    </div>
  );
}
