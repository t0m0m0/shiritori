# Issue #31: フロントエンドReact化 実装プラン

## Phase 1: Vite + React + TypeScript ビルド環境
- `frontend/` ディレクトリにViteプロジェクト作成
- ビルド出力先を `srv/static/dist/` に設定
- Go側で `srv/static/dist/` を配信 + fallback index.html

## Phase 2: TypeScript型定義 (`frontend/src/types/`)
- WebSocketメッセージ型 (送信14種, 受信19種)
- ゲーム状態型 (Room, Player, Settings 等)

## Phase 3: カスタムhooks
- `useWebSocket` - WS接続管理、自動再接続、メッセージ送受信
- `useGameState` - useReducerベースの状態管理 (画面遷移、ルーム状態、ゲーム状態)

## Phase 4: コンポーネント
- App.tsx (画面ルーティング)
- Lobby/ (RoomList, CreateRoom, InviteCard)
- Room/ (WaitingRoom, PlayerList, Settings)
- Game/ (WordInput, Timer, WordHistory, LivesDisplay, TurnIndicator, PlayerSidebar)
- Vote/ (VotePanel)
- GameOver/ (ScoreBoard, HistoryPanel, SettingsPanel, ShareButtons)
- common/ (Toast, ThemeSwitcher)

## Phase 5: Go側変更
- embed.goにdist追加、server.goルーティング変更
- index.htmlはReactが生成するものを配信

## Phase 6: 旧ファイル削除・テスト
