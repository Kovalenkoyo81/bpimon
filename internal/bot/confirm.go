package bot

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

type pendingAction struct {
	code      string
	action    string
	expiresAt time.Time
}

type Confirmator struct {
	mu      sync.Mutex
	pending map[int64]*pendingAction
	ttl     time.Duration
}

func NewConfirmator(ttl time.Duration) *Confirmator {
	return &Confirmator{
		pending: make(map[int64]*pendingAction),
		ttl:     ttl,
	}
}

// Request generates a confirmation code for the given action and stores it.
// Returns the generated code to show to the user.
func (c *Confirmator) Request(userID int64, action string, digits int) string {
	max := int64(1)
	for range digits {
		max *= 10
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(max))
	code := fmt.Sprintf("%0*d", digits, n)
	c.mu.Lock()
	c.pending[userID] = &pendingAction{
		code:      code,
		action:    action,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
	return code
}

// Validate checks if the input matches a pending confirmation for the user.
// Returns the action name and true on success, empty string and false otherwise.
func (c *Confirmator) Validate(userID int64, input string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for id, p := range c.pending {
		if now.After(p.expiresAt) {
			delete(c.pending, id)
		}
	}
	p, ok := c.pending[userID]
	if !ok {
		return "", false
	}
	if time.Now().After(p.expiresAt) {
		delete(c.pending, userID)
		return "", false
	}
	if strings.TrimSpace(input) != p.code {
		return "", false
	}
	action := p.action
	delete(c.pending, userID)
	return action, true
}
