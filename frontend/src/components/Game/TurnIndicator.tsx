interface Props {
  currentTurn: string;
  myName: string;
  turnOrder: string[];
}

export function TurnIndicator({ currentTurn, myName, turnOrder }: Props) {
  const isMyTurn = currentTurn === myName;

  return (
    <div className={`turn-indicator ${isMyTurn ? 'my-turn' : 'other-turn'}`}>
      <span>
        {isMyTurn ? '🎯 あなたの番です！' : `⏳ ${currentTurn}さんの番です`}
      </span>
      {turnOrder.length > 1 && (
        <div className="turn-order-list">
          {turnOrder.map((n) => (
            <span key={n} className={`turn-order-item${n === currentTurn ? ' active' : ''}`}>
              {n}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
