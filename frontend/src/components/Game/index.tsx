import { useCallback } from 'react';
import type { OutgoingMessage } from '../../types/messages';
import type { GameState, Action } from '../../hooks/useGameState';
import { RuleBadges } from '../common/RuleBadges';
import { WaitingRoom } from '../Room/WaitingRoom';
import { TurnIndicator } from './TurnIndicator';
import { LivesDisplay } from './LivesDisplay';
import { CurrentWord } from './CurrentWord';
import { Timer } from './Timer';
import { WordInput } from './WordInput';
import { WordHistory } from './WordHistory';
import { PlayerSidebar } from './PlayerSidebar';
import { getRoomLink, copyText } from '../../utils/helpers';

interface Props {
  state: GameState;
  dispatch: React.Dispatch<Action>;
  onSend: (msg: OutgoingMessage) => void;
}

export function GameRoom({ state, dispatch, onSend }: Props) {
  const handleLeave = useCallback(() => {
    onSend({ type: 'leave_room' });
    dispatch({ type: 'LEAVE_ROOM' });
  }, [onSend, dispatch]);

  const handleCopyLink = useCallback(async () => {
    const link = getRoomLink(state.currentRoomId);
    const ok = await copyText(link);
    dispatch({ type: 'ADD_MESSAGE', text: ok ? 'ルームのリンクをコピーしました' : 'コピーに失敗しました', msgType: ok ? 'success' : 'error' });
  }, [state.currentRoomId, dispatch]);

  const handleShareX = useCallback(() => {
    const link = getRoomLink(state.currentRoomId);
    const title = state.currentSettings.name || 'ルーム';
    const text = `しりとりで遊ぼう！「${title}」に参加してね🎮\n`;
    window.open(`https://x.com/intent/tweet?text=${encodeURIComponent(text)}&url=${encodeURIComponent(link)}`, '_blank', 'width=550,height=420');
  }, [state.currentRoomId, state.currentSettings.name]);

  const handleShareLINE = useCallback(() => {
    const link = getRoomLink(state.currentRoomId);
    const title = state.currentSettings.name || 'ルーム';
    const text = `しりとりで遊ぼう！「${title}」に参加してね🎮\n${link}`;
    window.open(`https://social-plugins.line.me/lineit/share?url=${encodeURIComponent(link)}&text=${encodeURIComponent(text)}`, '_blank', 'width=550,height=420');
  }, [state.currentRoomId, state.currentSettings.name]);

  return (
    <div className="container">
      <div className="game-header">
        <div className="room-title">{state.currentSettings.name || 'ルーム'}</div>
        <div className="game-rules">
          <RuleBadges settings={state.currentSettings} />
        </div>
        <div className="game-header-actions">
          <button className="share-btn share-btn-x" style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem' }} onClick={handleShareX}>
            <span className="share-icon">𝕏</span>
          </button>
          <button className="share-btn share-btn-line" style={{ padding: '0.3rem 0.6rem', fontSize: '0.75rem' }} onClick={handleShareLINE}>
            <span className="share-icon">💬</span>
          </button>
          <button className="btn btn-outline" style={{ padding: '0.35rem 0.8rem', fontSize: '0.8rem' }} onClick={handleCopyLink}>
            🔗 コピー
          </button>
          <button className="btn btn-outline" style={{ padding: '0.35rem 0.8rem', fontSize: '0.8rem' }} onClick={handleLeave}>
            退出
          </button>
        </div>
      </div>

      {!state.isPlaying ? (
        <WaitingRoom
          waitingPlayers={state.waitingPlayers}
          roomOwner={state.roomOwner}
          myName={state.myName}
          onSend={onSend}
        />
      ) : (
        <div>
          <TurnIndicator currentTurn={state.currentTurn} myName={state.myName} turnOrder={state.turnOrder} />
          <LivesDisplay currentLives={state.currentLives} myName={state.myName} maxLives={state.maxLives} />
          <CurrentWord word={state.currentWord} />
          <Timer seconds={state.timerSeconds} max={state.timerMax} />
          <WordInput
            isMyTurn={state.currentTurn === state.myName}
            currentTurn={state.currentTurn}
            currentWord={state.currentWord}
            isVoteActive={state.isVoteActive}
            lastWordPlayer={state.lastWordPlayer}
            myName={state.myName}
            onSend={onSend}
          />
          <div className="game-body">
            <WordHistory history={state.history} />
            <PlayerSidebar
              players={state.players}
              myName={state.myName}
              currentTurn={state.currentTurn}
              maxLives={state.maxLives}
              messages={state.messages}
            />
          </div>
        </div>
      )}
    </div>
  );
}
