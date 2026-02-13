interface Props {
  word: string;
}

export function CurrentWord({ word }: Props) {
  return (
    <div className="current-word-area">
      <div className="current-word-label">現在のことば</div>
      <div className="current-word">{word || '―'}</div>
    </div>
  );
}
