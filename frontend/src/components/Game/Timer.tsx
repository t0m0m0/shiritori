interface Props {
  seconds: number;
  max: number;
}

export function Timer({ seconds, max }: Props) {
  if (max <= 0) return null;
  const pct = max > 0 ? (seconds / max) * 100 : 100;
  const timerClass = seconds <= 5 ? 'danger' : seconds <= 10 ? 'warning' : '';

  return (
    <div>
      <div className={`timer-text ${timerClass}`}>{seconds}秒</div>
      <div className="timer-bar">
        <div className={`timer-bar-inner ${timerClass}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
