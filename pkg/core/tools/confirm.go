package tools

import (
	"sync"
	"time"
)

// TimeoutCallback is called when a confirmation request times out.
// This allows the TUI to be notified and exit confirmation mode.
type TimeoutCallback func()

// ConfirmationManager handles thread-safe channel-based communication
// between tools that require user confirmation and the TUI.
type ConfirmationManager struct {
	mu              sync.Mutex
	responseChan    chan bool
	pending         bool
	timeout         time.Duration
	timeoutCallback TimeoutCallback
}

// NewConfirmationManager creates a new ConfirmationManager with default timeout.
func NewConfirmationManager() *ConfirmationManager {
	return &ConfirmationManager{
		responseChan: make(chan bool, 1),
		pending:      false,
		timeout:      5 * time.Minute, // Prevent deadlock
	}
}

// SetTimeoutCallback sets the callback to invoke when a confirmation times out.
// This allows the TUI to exit confirmation mode on timeout.
func (cm *ConfirmationManager) SetTimeoutCallback(callback TimeoutCallback) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.timeoutCallback = callback
}

// SetTimeout sets the confirmation timeout duration.
func (cm *ConfirmationManager) SetTimeout(timeout time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.timeout = timeout
}

// RequestConfirmation blocks until the user responds or timeout occurs.
// Returns true if approved, false if rejected or timed out.
// If a timeout callback is set, it will be called when timeout occurs.
func (cm *ConfirmationManager) RequestConfirmation() bool {
	cm.mu.Lock()
	cm.pending = true
	timeout := cm.timeout
	// Clear any stale responses
	select {
	case <-cm.responseChan:
	default:
	}
	cm.mu.Unlock()

	// Wait for response with timeout
	select {
	case approved := <-cm.responseChan:
		cm.mu.Lock()
		cm.pending = false
		cm.mu.Unlock()
		return approved
	case <-time.After(timeout):
		cm.mu.Lock()
		cm.pending = false
		callback := cm.timeoutCallback
		cm.mu.Unlock()
		// Notify TUI that timeout occurred (outside of lock to prevent deadlock)
		if callback != nil {
			callback()
		}
		return false // Timeout = reject
	}
}

// SendResponse sends the user's response to the waiting tool.
// Called by the TUI when the user presses y/n.
func (cm *ConfirmationManager) SendResponse(approved bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.pending {
		// Non-blocking send in case the tool has timed out
		select {
		case cm.responseChan <- approved:
		default:
		}
	}
}

// IsPending returns whether a confirmation request is waiting for response.
func (cm *ConfirmationManager) IsPending() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.pending
}

// Cancel cancels any pending confirmation request.
// Used when the user quits the application during confirmation.
func (cm *ConfirmationManager) Cancel() {
	cm.SendResponse(false)
}
