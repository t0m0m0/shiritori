interface Props {
  currentLives: Record<string, number>;
  myName: string;
  maxLives: number;
}

export function LivesDisplay({ currentLives, myName, maxLives }: Props) {
  const myLivesCount = currentLives[myName] !== undefined ? currentLives[myName] : maxLives;

  return (
    <div className="my-lives-display">
      <span className="my-lives-label">ライフ</span>
      <div className="my-lives-hearts">
        {Array.from({ length: maxLives }, (_, i) => (
          <span key={i} className={`heart${i >= myLivesCount ? ' lost' : ''}`}>
            {i < myLivesCount ? '❤️' : '🤍'}
          </span>
        ))}
      </div>
    </div>
  );
}
