package srv

import (
	"sync"
	"time"
)

// TimerManager manages the turn countdown timer for a room.
type TimerManager struct {
	mu        sync.Mutex
	timeLimit int
	left      int
	cancel    chan struct{}
	onTick    func(timeLeft int)         // called each second
	onExpired func()                     // called when timer reaches 0
}

// NewTimerManager creates a new TimerManager.
// onTick is called every second with the remaining time.
// onExpired is called when the timer reaches 0.
func NewTimerManager(onTick func(int), onExpired func()) *TimerManager {
	return &TimerManager{
		onTick:    onTick,
		onExpired: onExpired,
	}
}

// Start begins the countdown with the given time limit (in seconds).
func (tm *TimerManager) Start(timeLimit int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.timeLimit = timeLimit
	if timeLimit <= 0 {
		return
	}
	tm.left = timeLimit
	tm.cancel = make(chan struct{})
	go tm.run()
}

// Reset resets the countdown to the configured time limit.
func (tm *TimerManager) Reset() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.timeLimit > 0 {
		tm.left = tm.timeLimit
	}
}

// Stop cancels the running timer.
func (tm *TimerManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.stopLocked()
}

func (tm *TimerManager) stopLocked() {
	if tm.cancel != nil {
		select {
		case <-tm.cancel:
		default:
			close(tm.cancel)
		}
		tm.cancel = nil
	}
}

// TimeLeft returns the remaining seconds.
func (tm *TimerManager) TimeLeft() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.left
}

func (tm *TimerManager) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-tm.cancel:
			return
		case <-ticker.C:
			tm.mu.Lock()
			tm.left--
			left := tm.left
			if left <= 0 {
				tm.cancel = nil
				tm.mu.Unlock()
				if tm.onExpired != nil {
					tm.onExpired()
				}
				return
			}
			tm.mu.Unlock()
			if tm.onTick != nil {
				tm.onTick(left)
			}
		}
	}
}
