package srv

import (
	"fmt"
	"sync"
	"time"
)

const (
	// defaultMaxLives is the default number of lives per player when not configured.
	defaultMaxLives = 3
	// voteTimeout is how long players have to vote before auto-resolution.
	voteTimeout = 15 * time.Second
)

// GameEngine manages game state: word validation, turns, scores, and lives.
type GameEngine struct {
	mu          sync.Mutex
	Settings    RoomSettings
	History     []WordEntry
	CurrentWord string
	UsedWords   map[string]bool
	TurnOrder   []string
	TurnIndex   int
	Players     map[string]*PlayerState // game-level state per player

	// resetTimer is called after a word is applied to reset the turn timer.
	resetTimer func()
}

// PlayerState holds per-player game state (score, lives).
type PlayerState struct {
	Score int
	Lives int
}

// NewGameEngine creates a GameEngine from settings and player names.
// ownerName goes first in turn order; the rest are appended in the given order.
func NewGameEngine(settings RoomSettings, turnOrder []string, resetTimer func()) *GameEngine {
	maxLives := settings.MaxLives
	if maxLives <= 0 {
		maxLives = defaultMaxLives
	}
	players := make(map[string]*PlayerState, len(turnOrder))
	for _, name := range turnOrder {
		players[name] = &PlayerState{Score: 0, Lives: maxLives}
	}
	return &GameEngine{
		Settings:    settings,
		History:     []WordEntry{},
		CurrentWord: "",
		UsedWords:   make(map[string]bool),
		TurnOrder:   turnOrder,
		TurnIndex:   0,
		Players:     players,
		resetTimer:  resetTimer,
	}
}

// AddPlayer adds a player mid-game with full lives.
func (ge *GameEngine) AddPlayer(name string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	maxLives := ge.Settings.MaxLives
	if maxLives <= 0 {
		maxLives = defaultMaxLives
	}
	ge.Players[name] = &PlayerState{Score: 0, Lives: maxLives}
	ge.TurnOrder = append(ge.TurnOrder, name)
}

// RemovePlayer removes a player from the game engine.
func (ge *GameEngine) RemovePlayer(name string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	delete(ge.Players, name)
	for i, n := range ge.TurnOrder {
		if n == name {
			ge.TurnOrder = append(ge.TurnOrder[:i], ge.TurnOrder[i+1:]...)
			if len(ge.TurnOrder) > 0 && ge.TurnIndex >= len(ge.TurnOrder) {
				ge.TurnIndex = 0
			}
			break
		}
	}
}

// ValidateResult represents the outcome of word validation.
type ValidateResult int

const (
	ValidateOK       ValidateResult = iota // Word accepted
	ValidateRejected                       // Word rejected (hard fail)
	ValidateVote                           // Need genre vote
	ValidatePenalty                        // Word rejected but player loses a life
)

// ValidateAndSubmitWord checks a word and applies it if valid.
func (ge *GameEngine) ValidateAndSubmitWord(word, playerName string, hasVotePending bool) (ValidateResult, string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if hasVotePending {
		return ValidateRejected, "投票中です。投票が終わるまでお待ちください"
	}

	// Check it's this player's turn
	if len(ge.TurnOrder) > 0 && ge.TurnOrder[ge.TurnIndex] != playerName {
		return ValidateRejected, fmt.Sprintf("%sさんの番です", ge.TurnOrder[ge.TurnIndex])
	}

	// Check player is not eliminated
	if ps, ok := ge.Players[playerName]; ok && ps.Lives <= 0 {
		return ValidateRejected, "あなたは脱落済みです"
	}

	// Check that word is valid Japanese kana
	if !isJapanese(word) {
		return ValidateRejected, "ひらがな・カタカナで入力してください"
	}

	hiragana := toHiragana(word)

	// Check length
	wlen := charCount(hiragana)
	if ge.Settings.MinLen > 0 && wlen < ge.Settings.MinLen {
		return ValidateRejected, fmt.Sprintf("%d文字以上で入力してください", ge.Settings.MinLen)
	}
	if ge.Settings.MaxLen > 0 && wlen > ge.Settings.MaxLen {
		return ValidateRejected, fmt.Sprintf("%d文字以下で入力してください", ge.Settings.MaxLen)
	}

	// Check first char matches last char of current word (skip for first word)
	if ge.CurrentWord != "" {
		prevHiragana := toHiragana(ge.CurrentWord)
		lastChar := getLastChar(prevHiragana)
		firstChar := getFirstChar(hiragana)
		if lastChar != firstChar {
			return ValidateRejected, fmt.Sprintf("「%c」から始まる言葉を入力してください", lastChar)
		}
	}

	// Check not already used — penalty
	if ge.UsedWords[hiragana] {
		ge.applyPenaltyLocked(playerName)
		return ValidatePenalty, "この言葉はすでに使われています"
	}

	// Check ends with ん
	runes := []rune(hiragana)
	if runes[len(runes)-1] == 'ん' {
		ge.applyPenaltyLocked(playerName)
		return ValidatePenalty, "「ん」で終わる言葉を使いました"
	}

	// Check no dakuten/handakuten
	if ge.Settings.NoDakuten {
		if badChar := ValidateNoDakuten(hiragana); badChar != 0 {
			ge.applyPenaltyLocked(playerName)
			return ValidatePenalty, fmt.Sprintf("「%c」は濁音・半濁音の文字です（濁音・半濁音禁止ルール）", badChar)
		}
	}

	// Check allowed rows
	if len(ge.Settings.AllowedRows) > 0 {
		if badChar, badRow := ValidateAllowedRows(hiragana, ge.Settings.AllowedRows); badChar != 0 {
			ge.applyPenaltyLocked(playerName)
			return ValidatePenalty, fmt.Sprintf("「%c」は%sの文字です（使用可能な行: %s）", badChar, badRow, formatAllowedRows(ge.Settings.AllowedRows))
		}
	}

	// All good — apply the word
	ge.applyWordLocked(word, hiragana, playerName)
	return ValidateOK, ""
}

// ApplyWord applies an accepted word (used by vote resolution). Acquires lock.
func (ge *GameEngine) ApplyWord(word, hiragana, playerName string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.applyWordLocked(word, hiragana, playerName)
}

func (ge *GameEngine) applyWordLocked(word, hiragana, playerName string) {
	ge.UsedWords[hiragana] = true
	ge.CurrentWord = word
	ge.History = append(ge.History, WordEntry{
		Word:   word,
		Player: playerName,
		Time:   time.Now().Format(time.RFC3339),
	})

	// Award point
	if ps, ok := ge.Players[playerName]; ok {
		ps.Score++
	}

	// Advance turn, skipping eliminated players
	if len(ge.TurnOrder) > 0 {
		start := ge.TurnIndex
		for {
			ge.TurnIndex = (ge.TurnIndex + 1) % len(ge.TurnOrder)
			if ge.TurnIndex == start {
				break
			}
			nextName := ge.TurnOrder[ge.TurnIndex]
			if ps, ok := ge.Players[nextName]; ok && ps.Lives > 0 {
				break
			}
		}
	}

	// Reset timer
	if ge.resetTimer != nil {
		ge.resetTimer()
	}
}

func (ge *GameEngine) applyPenaltyLocked(playerName string) {
	if ps, ok := ge.Players[playerName]; ok {
		ps.Lives--
	}
}

// ApplyPenalty decrements a player's lives. Acquires lock.
func (ge *GameEngine) ApplyPenalty(playerName string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	ge.applyPenaltyLocked(playerName)
}

// RevertWord reverts the last word (used when a challenge is upheld).
func (ge *GameEngine) RevertWord(word, playerName string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if len(ge.History) > 0 {
		ge.History = ge.History[:len(ge.History)-1]
	}
	delete(ge.UsedWords, toHiragana(word))

	// Revert score
	if ps, ok := ge.Players[playerName]; ok {
		if ps.Score > 0 {
			ps.Score--
		}
	}

	prevWord := ""
	if len(ge.History) > 0 {
		prevWord = ge.History[len(ge.History)-1].Word
	}

	// Turn stays with the original player
	for i, name := range ge.TurnOrder {
		if name == playerName {
			ge.TurnIndex = i
			break
		}
	}

	// Penalize
	ge.applyPenaltyLocked(playerName)

	ge.CurrentWord = prevWord
	if ge.resetTimer != nil {
		ge.resetTimer()
	}
}

// GetAlivePlayers returns names of players with lives > 0.
func (ge *GameEngine) GetAlivePlayers() []string {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	var alive []string
	for name, ps := range ge.Players {
		if ps.Lives > 0 {
			alive = append(alive, name)
		}
	}
	return alive
}

// CheckElimination checks if a player is eliminated and whether the game is over.
func (ge *GameEngine) CheckElimination(playerName string, totalPlayers int) (eliminated bool, gameOver bool, lastSurvivor string) {
	ge.mu.Lock()
	defer ge.mu.Unlock()

	if ps, ok := ge.Players[playerName]; ok && ps.Lives <= 0 {
		eliminated = true
	}
	var alive []string
	for name, ps := range ge.Players {
		if ps.Lives > 0 {
			alive = append(alive, name)
		}
	}
	if totalPlayers <= 1 {
		if len(alive) == 0 {
			gameOver = true
		}
	} else {
		if len(alive) <= 1 {
			gameOver = true
			if len(alive) == 1 {
				lastSurvivor = alive[0]
			}
		}
	}
	return
}

// GetScores returns a map of player name -> score.
func (ge *GameEngine) GetScores() map[string]int {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	scores := make(map[string]int, len(ge.Players))
	for name, ps := range ge.Players {
		scores[name] = ps.Score
	}
	return scores
}

// GetLives returns a map of player name -> lives.
func (ge *GameEngine) GetLives() map[string]int {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	lives := make(map[string]int, len(ge.Players))
	for name, ps := range ge.Players {
		lives[name] = ps.Lives
	}
	return lives
}

// GetPlayerLives returns a specific player's remaining lives.
func (ge *GameEngine) GetPlayerLives(name string) int {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	if ps, ok := ge.Players[name]; ok {
		return ps.Lives
	}
	return 0
}

// MaxLives returns the configured max lives.
func (ge *GameEngine) MaxLives() int {
	maxLives := ge.Settings.MaxLives
	if maxLives <= 0 {
		maxLives = defaultMaxLives
	}
	return maxLives
}

// CurrentTurn returns the name of the player whose turn it is.
func (ge *GameEngine) CurrentTurn() string {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	if len(ge.TurnOrder) > 0 && ge.TurnIndex < len(ge.TurnOrder) {
		return ge.TurnOrder[ge.TurnIndex]
	}
	return ""
}

// Snapshot returns a copy of the current game state.
func (ge *GameEngine) Snapshot() (history []WordEntry, currentWord string, turnOrder []string, turnIndex int) {
	ge.mu.Lock()
	defer ge.mu.Unlock()
	history = make([]WordEntry, len(ge.History))
	copy(history, ge.History)
	currentWord = ge.CurrentWord
	turnOrder = make([]string, len(ge.TurnOrder))
	copy(turnOrder, ge.TurnOrder)
	turnIndex = ge.TurnIndex
	return
}
