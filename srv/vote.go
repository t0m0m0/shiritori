package srv

import (
	"fmt"
	"sync"
)

// PendingVote holds state for an in-progress genre vote.
type PendingVote struct {
	Word       string
	Hiragana   string
	Player     string
	Challenger string
	Votes      map[string]bool // player name -> accept (true) / reject (false)
	Type       string          // "genre" or "challenge"
	Reason     string
	Resolved   bool
}

// VoteResolution is the outcome of a vote.
type VoteResolution struct {
	Type       string
	Word       string
	Player     string
	Challenger string
	Accepted   bool
	Reverted   bool
}

// VoteInfo describes a new vote request.
type VoteInfo struct {
	Type       string
	Word       string
	Player     string
	Challenger string
	Reason     string
	VoteCount  int
	Total      int
}

// VoteManager manages voting and challenge logic for a room.
type VoteManager struct {
	mu          sync.Mutex
	pendingVote *PendingVote

	// playerExists checks if a player name is in the room.
	playerExists func(name string) bool
	// playerCount returns the number of players.
	playerCount func() int
}

// NewVoteManager creates a new VoteManager.
func NewVoteManager(playerExists func(string) bool, playerCount func() int) *VoteManager {
	return &VoteManager{
		playerExists: playerExists,
		playerCount:  playerCount,
	}
}

// HasPendingVote returns true if there is an unresolved vote.
func (vm *VoteManager) HasPendingVote() bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.pendingVote != nil && !vm.pendingVote.Resolved
}

// GetPending returns the current pending vote (may be nil).
func (vm *VoteManager) GetPending() *PendingVote {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	return vm.pendingVote
}

// Clear removes any pending vote.
func (vm *VoteManager) Clear() {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.pendingVote = nil
}

// StartChallengeVote starts a vote to challenge the last word.
func (vm *VoteManager) StartChallengeVote(challengerName string, lastWord WordEntry, playerExists func(string) bool) (VoteInfo, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.pendingVote != nil && !vm.pendingVote.Resolved {
		return VoteInfo{}, fmt.Errorf("投票中です。投票が終わるまでお待ちください")
	}
	if !playerExists(challengerName) {
		return VoteInfo{}, fmt.Errorf("ルームに参加していません")
	}
	if lastWord.Player == challengerName {
		return VoteInfo{}, fmt.Errorf("自分の単語には指摘できません")
	}

	hiragana := toHiragana(lastWord.Word)
	vm.pendingVote = &PendingVote{
		Word:       lastWord.Word,
		Hiragana:   hiragana,
		Player:     lastWord.Player,
		Challenger: challengerName,
		Votes:      make(map[string]bool),
		Type:       "challenge",
		Reason:     fmt.Sprintf("「%s」は存在しない単語かもしれません", lastWord.Word),
	}

	// Challenger auto-votes reject (word should be removed)
	vm.pendingVote.Votes[challengerName] = false

	info := VoteInfo{
		Type:       "challenge",
		Word:       lastWord.Word,
		Player:     lastWord.Player,
		Challenger: challengerName,
		Reason:     vm.pendingVote.Reason,
		VoteCount:  len(vm.pendingVote.Votes),
		Total:      vm.countEligibleVotersLocked(),
	}
	return info, nil
}

// CastVote records a player's vote and returns resolution if all votes are in.
func (vm *VoteManager) CastVote(playerName string, accept bool) (resolved bool, result VoteResolution) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.pendingVote == nil || vm.pendingVote.Resolved {
		return false, VoteResolution{}
	}

	if !vm.playerExists(playerName) {
		return false, VoteResolution{}
	}

	// The challenged player cannot vote
	if vm.pendingVote.Type == "challenge" && vm.pendingVote.Player == playerName {
		return false, VoteResolution{}
	}

	vm.pendingVote.Votes[playerName] = accept

	eligibleVoters := vm.countEligibleVotersLocked()
	if len(vm.pendingVote.Votes) < eligibleVoters {
		return false, VoteResolution{}
	}

	return vm.resolveVoteLocked()
}

// ForceResolveVote resolves the vote by timeout (majority wins, tie = reject).
func (vm *VoteManager) ForceResolveVote() (resolved bool, result VoteResolution) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.pendingVote == nil || vm.pendingVote.Resolved {
		return false, VoteResolution{}
	}

	return vm.resolveVoteLocked()
}

// WithdrawChallenge allows the challenger to withdraw their challenge.
func (vm *VoteManager) WithdrawChallenge(challengerName string) bool {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.pendingVote == nil || vm.pendingVote.Resolved {
		return false
	}
	if vm.pendingVote.Type != "challenge" {
		return false
	}
	if vm.pendingVote.Challenger != challengerName {
		return false
	}

	vm.pendingVote.Resolved = true
	vm.pendingVote = nil
	return true
}

// VoteCount returns the current vote count and total eligible voters.
func (vm *VoteManager) VoteCount() (count int, total int) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	if vm.pendingVote != nil {
		count = len(vm.pendingVote.Votes)
	}
	total = vm.countEligibleVotersLocked()
	return
}

// countEligibleVotersLocked returns the number of players who can vote.
// Caller must hold vm.mu.
func (vm *VoteManager) countEligibleVotersLocked() int {
	total := vm.playerCount()
	if vm.pendingVote != nil && vm.pendingVote.Type == "challenge" {
		if vm.playerExists(vm.pendingVote.Player) {
			total--
		}
	}
	return total
}

func (vm *VoteManager) resolveVoteLocked() (resolved bool, result VoteResolution) {
	vm.pendingVote.Resolved = true
	acceptCount := 0
	rejectCount := 0
	for _, v := range vm.pendingVote.Votes {
		if v {
			acceptCount++
		} else {
			rejectCount++
		}
	}
	eligibleVoters := vm.countEligibleVotersLocked()
	rejectCount += eligibleVoters - len(vm.pendingVote.Votes)

	accepted := acceptCount > rejectCount
	result = VoteResolution{
		Type:       vm.pendingVote.Type,
		Word:       vm.pendingVote.Word,
		Player:     vm.pendingVote.Player,
		Challenger: vm.pendingVote.Challenger,
		Accepted:   accepted,
	}

	// For genre votes
	if vm.pendingVote.Type == "genre" {
		if !accepted {
			vm.pendingVote = nil
		}
		return true, result
	}

	// Challenge: not accepted means revert
	if !accepted {
		result.Reverted = true
	}

	vm.pendingVote = nil
	return true, result
}
