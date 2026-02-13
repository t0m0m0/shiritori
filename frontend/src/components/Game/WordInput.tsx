import { useRef, useCallback, useState } from 'react';
import type { OutgoingMessage } from '../../types/messages';
import { filterHiragana } from '../../utils/helpers';

interface Props {
  isMyTurn: boolean;
  currentTurn: string;
  currentWord: string;
  isVoteActive: boolean;
  lastWordPlayer: string;
  myName: string;
  onSend: (msg: OutgoingMessage) => void;
}

export function WordInput({ isMyTurn, currentTurn, currentWord, isVoteActive, lastWordPlayer, myName, onSend }: Props) {
  const [value, setValue] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);
  const composingRef = useRef(false);

  const handleInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (composingRef.current) {
      setValue(e.target.value);
      return;
    }
    setValue(filterHiragana(e.target.value));
  }, []);

  const handleCompositionEnd = useCallback((e: React.CompositionEvent<HTMLInputElement>) => {
    composingRef.current = false;
    setValue(filterHiragana((e.target as HTMLInputElement).value));
  }, []);

  const submit = useCallback(() => {
    const word = value.trim();
    if (!word) return;
    onSend({ type: 'answer', word });
    setValue('');
    inputRef.current?.focus();
  }, [value, onSend]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') { e.preventDefault(); submit(); }
  }, [submit]);

  const handleChallenge = useCallback(() => {
    if (isVoteActive || lastWordPlayer === myName) return;
    onSend({ type: 'challenge' });
  }, [isVoteActive, lastWordPlayer, myName, onSend]);

  const isFirstWord = currentWord === '―' || currentWord === '';
  const canChallenge = currentWord !== '―' && currentWord !== '' && !isVoteActive && lastWordPlayer !== myName;

  return (
    <div className={`answer-area${!isMyTurn ? ' disabled' : ''}`}>
      <input
        ref={inputRef}
        type="text"
        value={value}
        onChange={handleInput}
        onCompositionStart={() => { composingRef.current = true; }}
        onCompositionEnd={handleCompositionEnd}
        onKeyDown={handleKeyDown}
        placeholder={isMyTurn ? (isFirstWord ? '最初のことばを入力…' : 'ことばを入力…') : `${currentTurn}さんの番です…`}
        disabled={!isMyTurn}
        autoComplete="off"
        lang="ja"
      />
      <button className="btn btn-primary" onClick={submit}>送信</button>
      <button className="btn btn-outline" onClick={handleChallenge} disabled={!canChallenge}>
        ⚠️ 指摘
      </button>
    </div>
  );
}
