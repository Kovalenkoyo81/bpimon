package alert

import (
	"bpimon/internal/log"
	"bpimon/internal/notifier"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	silencedUntil time.Time
	statePath     string
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
	}
}

func (m *Manager) Run(ctx context.Context) {
	if m.statePath != "" {
		m.loadState()
	}

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
			if m.statePath != "" {
				m.saveState()
			}
			return
		}
	}
}

// Resolve records an admin's answer for a currently active alert.
// name may be a prefix of the alert name (callback data is truncated).
func (m *Manager) Resolve(name string, r Resolution) error {
	name, err := m.lookupName(name)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.active[name] {
		return ErrNotActive
	}
	m.resolution[name] = r
	return nil
}

// lookupName finds the alert whose name equals or starts with prefix.
func (m *Manager) lookupName(prefix string) (string, error) {
	var found []string
	for _, a := range m.alerts {
		n := a.Name()
		if n == prefix {
			return n, nil
		}
		if strings.HasPrefix(n, prefix) {
			found = append(found, n)
		}
	}
	switch len(found) {
	case 0:
		return "", ErrNotActive
	case 1:
		return found[0], nil
	default:
		return "", ErrAmbiguous
	}
}

// checkAll runs every alert once and sends fire/recovery notifications.
// Alerts re-fire every cooldown while active. Askable alerts carry answer
// buttons until resolved; an Intentional answer silences them until the
// condition clears.
func (m *Manager) checkAll() {
	for _, a := range m.alerts {
		name := a.Name()
		verdict, msg := evaluate(a)
		if verdict == Unknown {
			continue // keep previous state: no false recovery, no false alarm
		}
		ok := verdict == Firing

		m.mu.Lock()
		active := m.active[name]
		lastSent := m.lastSent[name]
		res := m.resolution[name]
		m.mu.Unlock()

		askable, isAskable := a.(Askable)

		if ok {
			if res == Intentional {
				continue
			}
			due := time.Since(lastSent) >= m.cooldown
			if isAskable && !active {
				due = true // new incident: ask even within cooldown of the previous one
			}
			if !due {
				continue
			}
			text := "🚨 " + name + ": " + msg
			if isAskable && res == unresolved {
				m.sendQuestion(name, text, askable)
			} else {
				_ = m.notifier.SendAlert(text)
			}
			m.mu.Lock()
			m.lastSent[name] = time.Now()
			m.active[name] = true
			m.mu.Unlock()
		} else if active {
			if res != Intentional {
				_ = m.notifier.SendAlert("✅ " + name + ": back to normal")
			}
			m.mu.Lock()
			m.active[name] = false
			delete(m.resolution, name)
			m.mu.Unlock()
		}
	}
}

func (m *Manager) sendQuestion(name, text string, a Askable) {
	bs, ok := m.notifier.(notifier.ButtonSender)
	if !ok {
		_ = m.notifier.SendAlert(text)
		return
	}
	intentional, incident := a.Choices()
	_ = bs.SendAlertWithButtons(text, []notifier.Button{
		{Text: intentional, Data: CallbackData(name, Intentional)},
		{Text: incident, Data: CallbackData(name, Incident)},
	})
}

type managerState struct {
	Active     map[string]bool       `json:"active"`
	LastSent   map[string]time.Time  `json:"last_sent"`
	Resolution map[string]Resolution `json:"resolution,omitempty"`
}

func (m *Manager) loadState() {
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		return
	}
	var s managerState
	if err := json.Unmarshal(data, &s); err != nil {
		log.Warn.Printf("failed to load alert state from %s: %v", m.statePath, err)
		return
	}
	if s.Active != nil {
		m.active = s.Active
	}
	if s.LastSent != nil {
		m.lastSent = s.LastSent
	}
	if s.Resolution != nil {
		m.resolution = s.Resolution
	}
	log.Info.Printf("alert state loaded from %s", m.statePath)
}

func (m *Manager) saveState() {
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0750); err != nil {
		log.Error.Printf("failed to create state dir: %v", err)
		return
	}
	m.mu.Lock()
	s := managerState{Active: m.active, LastSent: m.lastSent, Resolution: m.resolution}
	data, err := json.Marshal(s)
	m.mu.Unlock()
	if err != nil {
		log.Error.Printf("failed to marshal alert state: %v", err)
		return
	}
	if err := os.WriteFile(m.statePath, data, 0600); err != nil {
		log.Error.Printf("failed to save alert state to %s: %v", m.statePath, err)
		return
	}
	log.Info.Printf("alert state saved to %s", m.statePath)
}
