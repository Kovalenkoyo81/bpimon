package alert

import (
	"bpimon/internal/log"
	"bpimon/internal/notifier"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
		alerts:   a,
		notifier: n,
		interval: interval,
		cooldown: cooldown,
		lastSent: make(map[string]time.Time),
		active:   make(map[string]bool),
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
			for _, a := range m.alerts {
				name := a.Name()
				ok, msg := a.Check()

				m.mu.Lock()
				active := m.active[name]
				lastSent := m.lastSent[name]
				m.mu.Unlock()

				if ok {
					if time.Since(lastSent) >= m.cooldown {
						_ = m.notifier.SendAlert("🚨 " + name + ": " + msg)
						m.mu.Lock()
						m.lastSent[name] = time.Now()
						m.active[name] = true
						m.mu.Unlock()
					}
				} else if active {
					_ = m.notifier.SendAlert("✅ " + name + ": back to normal")
					m.mu.Lock()
					m.active[name] = false
					m.mu.Unlock()
				}
			}
		case <-ctx.Done():
			if m.statePath != "" {
				m.saveState()
			}
			return
		}
	}
}

type managerState struct {
	Active   map[string]bool      `json:"active"`
	LastSent map[string]time.Time `json:"last_sent"`
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
	log.Info.Printf("alert state loaded from %s", m.statePath)
}

func (m *Manager) saveState() {
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0750); err != nil {
		log.Error.Printf("failed to create state dir: %v", err)
		return
	}
	s := managerState{Active: m.active, LastSent: m.lastSent}
	data, err := json.Marshal(s)
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
