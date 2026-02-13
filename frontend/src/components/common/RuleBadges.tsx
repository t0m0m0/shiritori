import type { RoomSettings } from '../../types/messages';

const DEFAULT_MAX_LIVES = 3;

interface Props {
  settings: RoomSettings;
  showPrivate?: boolean;
  owner?: string;
  playerCount?: number;
}

export function RuleBadges({ settings, showPrivate, owner, playerCount }: Props) {
  const s = settings;
  const badges: string[] = [];
  if (showPrivate && s.private) badges.push('🔒 プライベート');
  if (owner) badges.push(`👑 ホスト: ${owner}`);
  if (playerCount !== undefined) badges.push(`👥 ${playerCount}人`);
  if (s.genre) badges.push(`🏷️ ${s.genre}`);
  if (s.minLen > 1) badges.push(`最少${s.minLen}文字`);
  if (s.maxLen > 0) badges.push(`最大${s.maxLen}文字`);
  if (s.timeLimit > 0) badges.push(`⏱️ ${s.timeLimit}秒`);
  if (s.allowedRows && s.allowedRows.length > 0) badges.push(`🎯 ${s.allowedRows.join('・')}`);
  if (s.noDakuten) badges.push('🚫 濁音・半濁音禁止');
  badges.push(`❤️ ライフ${s.maxLives || DEFAULT_MAX_LIVES}`);

  return (
    <>
      {badges.map((b, i) => (
        <span key={i} className="rule-badge">{b}</span>
      ))}
    </>
  );
}
