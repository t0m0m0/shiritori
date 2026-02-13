import { useState, useEffect, useRef, useCallback } from 'react';
import type { OutgoingMessage } from '../../types/messages';

interface VoteState {
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
}

interface Props {
  vote: VoteState;
  myName: string;
  onSend: (msg: OutgoingMessage) => void;
  rebuttals: { player: string; text: string }[];
}

export function VotePanel({ vote, myName, onSend, rebuttals }: Props) {
  const [voteTimer, setVoteTimer] = useState(15);
  const [rebuttalText, setRebuttalText] = useState('');
  const [rebuttalSent, setRebuttalSent] = useState(false);
  const [hasVotedLocal, setHasVotedLocal] = useState(vote.hasVoted);
  const rebuttalRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setVoteTimer(15);
    setRebuttalText('');
    setRebuttalSent(false);
    setHasVotedLocal(vote.hasVoted);
    const interval = setInterval(() => {
      setVoteTimer((prev) => {
        if (prev <= 1) { clearInterval(interval); return 0; }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [vote.word, vote.voteType]);

  const isChallenge = vote.voteType === 'challenge';
  const isChallengedPlayer = isChallenge && vote.player === myName;
  const isChallenger = isChallenge && vote.challenger === myName;

  const handleVote = useCallback((accept: boolean) => {
    if (hasVotedLocal) return;
    setHasVotedLocal(true);
    onSend({ type: 'vote', accept });
  }, [hasVotedLocal, onSend]);

  const handleRebuttal = useCallback(() => {
    const text = rebuttalText.trim();
    if (!text) return;
    onSend({ type: 'rebuttal', rebuttal: text });
    setRebuttalText('');
    setRebuttalSent(true);
  }, [rebuttalText, onSend]);

  const handleWithdraw = useCallback(() => {
    onSend({ type: 'withdraw_challenge' });
  }, [onSend]);

  const pct = vote.totalPlayers > 0 ? (vote.voteCount / vote.totalPlayers) * 100 : 0;

  return (
    <div className="vote-overlay">
      <div className="vote-card">
        <h3>{isChallenge ? '🗳️ 単語指摘の投票' : '🗳️ ジャンル投票'}</h3>
        <p className="vote-question">
          {isChallenge
            ? `${vote.challenger}さんが「${vote.word}」を指摘しました`
            : `${vote.player}さんが「${vote.word}」を入力しました`}
        </p>
        <div className="vote-word">{vote.word}</div>
        <p className="vote-question">
          {isChallenge
            ? (vote.reason || 'この単語を認めますか？')
            : `ジャンル「${vote.genre}」のリストにない単語です。認めますか？`}
        </p>

        {/* Vote buttons / rebuttal / waiting */}
        {isChallengedPlayer ? (
          <div className="rebuttal-area">
            <p className="rebuttal-label">💬 反論メッセージを送れます：</p>
            <div className="rebuttal-input-row">
              <input
                ref={rebuttalRef}
                type="text"
                className="rebuttal-input"
                placeholder={rebuttalSent ? '送信済み ✓' : '反論を入力…'}
                maxLength={100}
                value={rebuttalText}
                onChange={(e) => setRebuttalText(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleRebuttal(); }}
                disabled={rebuttalSent}
              />
              <button className="btn btn-primary" onClick={handleRebuttal} disabled={rebuttalSent}>送信</button>
            </div>
            <p className="rebuttal-hint">他のプレイヤーに表示されます（投票には参加できません）</p>
          </div>
        ) : hasVotedLocal ? (
          <>
            <div className="vote-waiting">投票済み。他のプレイヤーの投票を待っています…</div>
            {isChallenger && (
              <div className="withdraw-area">
                <button className="btn btn-outline" onClick={handleWithdraw}>🔙 指摘を取り下げる</button>
              </div>
            )}
          </>
        ) : (
          <div className="vote-buttons">
            <button className="btn btn-accept" onClick={() => handleVote(true)}>⭕ 存在する</button>
            <button className="btn btn-reject" onClick={() => handleVote(false)}>❌ 存在しない</button>
          </div>
        )}

        {/* Rebuttal display */}
        {rebuttals.length > 0 && (
          <div className="rebuttal-display">
            {rebuttals.map((r, i) => (
              <div key={i} style={{ marginBottom: '0.3rem' }}>
                <span className="rebuttal-sender">{r.player}:</span> {r.text}
              </div>
            ))}
          </div>
        )}

        <div className="vote-progress">
          <span>{vote.voteCount} / {vote.totalPlayers} 投票済み</span>
          <div className="vote-progress-bar">
            <div className="vote-progress-bar-inner" style={{ width: `${pct}%` }} />
          </div>
        </div>
        <div className="vote-timer">
          {voteTimer > 0 ? `${voteTimer}秒で自動判定` : '判定中…'}
        </div>
      </div>
    </div>
  );
}
