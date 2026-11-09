package alert

import (
	"bpimon/internal/log"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type managerState struct {
	Active     map[string]bool       `json:"active"`
	LastSent   map[string]time.Time  `json:"last_sent"`
	Resolution map[string]Resolution `json:"resolution,omitempty"`
	Question   map[string]int        `json:"question_msg,omitempty"`
}

func (m *Manager) loadState() {
	if m.statePath == "" {
		return
	}
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Warn.Printf("failed to read alert state %s: %v", m.statePath, err)
		}
		return
	}
	var s managerState
	if err := json.Unmarshal(data, &s); err != nil {
		log.Warn.Printf("failed to load alert state from %s: %v", m.statePath, err)
		return
	}
	m.mu.Lock()
	if s.Active != nil {
		m.active = s.Active
	}
	if s.LastSent != nil {
		m.lastSent = s.LastSent
	}
	if s.Resolution != nil {
		m.resolution = s.Resolution
	}
	if s.Question != nil {
		m.question = s.Question
	}
	m.mu.Unlock()
	log.Info.Printf("alert state loaded from %s", m.statePath)
}

// saveState writes the state atomically (temp file + rename), so a crash
// mid-write never leaves a truncated file behind. Called on every change.
func (m *Manager) saveState() {
	if m.statePath == "" {
		return
	}
	m.mu.Lock()
	data, err := json.Marshal(managerState{
		Active:     m.active,
		LastSent:   m.lastSent,
		Resolution: m.resolution,
		Question:   m.question,
	})
	m.mu.Unlock()
	if err != nil {
		log.Error.Printf("failed to marshal alert state: %v", err)
		return
	}

	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	dir := filepath.Dir(m.statePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Error.Printf("failed to create state dir: %v", err)
		return
	}
	tmp, err := os.CreateTemp(dir, ".alert_state-*")
	if err != nil {
		log.Error.Printf("failed to save alert state: %v", err)
		return
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if werr != nil || serr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		log.Error.Printf("failed to save alert state: write=%v sync=%v close=%v", werr, serr, cerr)
		return
	}
	if err := os.Chmod(tmp.Name(), 0600); err != nil {
		log.Warn.Printf("chmod alert state: %v", err)
	}
	if err := os.Rename(tmp.Name(), m.statePath); err != nil {
		_ = os.Remove(tmp.Name())
		log.Error.Printf("failed to save alert state to %s: %v", m.statePath, err)
	}
}
