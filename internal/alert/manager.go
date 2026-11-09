package alert

import (
	"bpimon/internal/log"
	"bpimon/internal/notifier"
	"context"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	mu            sync.Mutex
	alerts        []Alert
	notifier      notifier.AlertSender
	interval      time.Duration
	cooldown      time.Duration
	lastSent      map[string]time.Time
	active        map[string]bool
	resolution    map[string]Resolution
	question      map[string]int // alert name → message ID of the latest question with buttons
	silencedUntil time.Time
	statePath     string
	saveMu        sync.Mutex // serialises state file writes
}

func (m *Manager) Silence(d time.Duration) {
	m.mu.Lock()
	m.silencedUntil = time.Now().Add(d)
	m.mu.Unlock()
}

func (m *Manager) Unsilence() {
	m.mu.Lock()
	m.silencedUntil = time.Time{}
	m.mu.Unlock()
}

func (m *Manager) IsSilenced() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Now().Before(m.silencedUntil)
}

func (m *Manager) SilencedUntil() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.silencedUntil
}

func (m *Manager) SetStatePath(path string) {
	m.statePath = path
}

func NewManager(a []Alert, n notifier.AlertSender, interval, cooldown time.Duration) *Manager {
	return &Manager{
		alerts:     a,
		notifier:   n,
		interval:   interval,
		cooldown:   cooldown,
		lastSent:   make(map[string]time.Time),
		active:     make(map[string]bool),
		resolution: make(map[string]Resolution),
		question:   make(map[string]int),
	}
}

// Run checks alerts every interval until ctx is cancelled. State is saved
// on every change, and once more before Run returns.
func (m *Manager) Run(ctx context.Context) {
	m.loadState()

	t := time.NewTicker(m.interval)
	defer t.Stop()

	wasSilenced := false

	for {
		select {
		case <-t.C:
			silenced := m.IsSilenced()
			if wasSilenced && !silenced {
				_ = m.notifier.SendAlert("🔔 Alert silence expired, monitoring resumed")
			}
			wasSilenced = silenced
			if silenced {
				continue
			}
			m.checkAll()
		case <-ctx.Done():
			m.saveState()
			return
		}
	}
}

// Resolve records an admin's answer. name may be a prefix of the alert
// name (callback data is truncated). If the manager does not consider the
// alert active (e.g. right after a restart, or a button on an old message),
// the condition is checked right now instead of trusting stale state.
func (m *Manager) Resolve(name string, r Resolution) error {
	a, err := m.lookup(name)
	if err != nil {
		return err
	}
	name = a.Name()

	m.mu.Lock()
	active := m.active[name]
	m.mu.Unlock()

	if !active {
		switch v, _ := evaluate(a); v {
		case Clear:
			return ErrNotActive
		case Unknown:
			return ErrUnknown
		}
	}

	m.mu.Lock()
	if !m.active[name] {
		m.active[name] = true
		m.lastSent[name] = time.Now()
	}
	m.resolution[name] = r
	msgID := m.question[name]
	delete(m.question, name)
	m.mu.Unlock()

	// The pressed message has its buttons removed by the caller; this
	// clears the latest question if a stale one was pressed instead.
	m.clearButtons(msgID)
	m.saveState()
	return nil
}

// lookup finds the alert whose name equals or starts with prefix.
func (m *Manager) lookup(prefix string) (Alert, error) {
	var found []Alert
	for _, a := range m.alerts {
		n := a.Name()
		if n == prefix {
			return a, nil
		}
		if strings.HasPrefix(n, prefix) {
			found = append(found, a)
		}
	}
	switch len(found) {
	case 0:
		return nil, ErrNotActive
	case 1:
		return found[0], nil
	default:
		return nil, ErrAmbiguous
	}
}

// checkAll runs every alert once and sends fire/recovery notifications.
// Alerts re-fire every cooldown while active. Askable alerts carry answer
// buttons until resolved — only on the latest question; an Intentional
// answer silences them until the condition clears.
func (m *Manager) checkAll() {
	changed := false
	for _, a := range m.alerts {
		if m.checkOne(a) {
			changed = true
		}
	}
	if changed {
		m.saveState()
	}
}

// checkOne evaluates a single alert and reports whether state changed.
func (m *Manager) checkOne(a Alert) bool {
	name := a.Name()
	verdict, msg := evaluate(a)
	if verdict == Unknown {
		return false // keep previous state: no false recovery, no false alarm
	}

	m.mu.Lock()
	active := m.active[name]
	lastSent := m.lastSent[name]
	res := m.resolution[name]
	m.mu.Unlock()

	askable, isAskable := a.(Askable)

	if verdict == Clear {
		if !active {
			return false
		}
		if res != Intentional {
			_ = m.notifier.SendAlert("✅ " + name + ": back to normal")
		}
		m.mu.Lock()
		m.active[name] = false
		delete(m.resolution, name)
		msgID := m.question[name]
		delete(m.question, name)
		m.mu.Unlock()
		m.clearButtons(msgID)
		return true
	}

	if res == Intentional {
		return false
	}
	due := time.Since(lastSent) >= m.cooldown
	if isAskable && !active {
		due = true // new incident: ask even within cooldown of the previous one
	}
	if !due {
		return false
	}

	text := "🚨 " + name + ": " + msg
	newMsg := 0
	if isAskable && res == unresolved {
		newMsg = m.sendQuestion(name, text, askable)
	} else {
		_ = m.notifier.SendAlert(text)
	}

	m.mu.Lock()
	m.lastSent[name] = time.Now()
	m.active[name] = true
	oldMsg := m.question[name]
	if newMsg != 0 {
		m.question[name] = newMsg
	}
	m.mu.Unlock()
	if newMsg != 0 {
		m.clearButtons(oldMsg) // only the latest question keeps its buttons
	}
	return true
}

// sendQuestion sends text with answer buttons and returns the message ID
// (0 if buttons are unsupported or sending failed).
func (m *Manager) sendQuestion(name, text string, a Askable) int {
	bs, ok := m.notifier.(notifier.ButtonSender)
	if !ok {
		_ = m.notifier.SendAlert(text)
		return 0
	}
	intentional, incident := a.Choices()
	id, err := bs.SendAlertWithButtons(text, []notifier.Button{
		{Text: intentional, Data: CallbackData(name, Intentional)},
		{Text: incident, Data: CallbackData(name, Incident)},
	})
	if err != nil {
		log.Warn.Printf("send question for %s: %v", name, err)
		return 0
	}
	return id
}

func (m *Manager) clearButtons(msgID int) {
	if msgID == 0 {
		return
	}
	bs, ok := m.notifier.(notifier.ButtonSender)
	if !ok {
		return
	}
	if err := bs.ClearButtons(msgID); err != nil {
		log.Warn.Printf("clear buttons on message %d: %v", msgID, err)
	}
}
