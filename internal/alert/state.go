package alert

import (
	"log/slog"
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
			slog.Warn("read alert state failed", "path", m.statePath, "err", err)
		}
		return
	}
	var s managerState
	if err := json.Unmarshal(data, &s); err != nil {
		slog.Warn("parse alert state failed", "path", m.statePath, "err", err)
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
	slog.Info("alert state loaded", "path", m.statePath)
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
		slog.Error("marshal alert state failed", "err", err)
		return
	}

	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	dir := filepath.Dir(m.statePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		slog.Error("create state dir failed", "err", err)
		return
	}
	tmp, err := os.CreateTemp(dir, ".alert_state-*")
	if err != nil {
		slog.Error("save alert state failed", "err", err)
		return
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if werr != nil || serr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		slog.Error("save alert state failed", "write_err", werr, "sync_err", serr, "close_err", cerr)
		return
	}
	if err := os.Chmod(tmp.Name(), 0600); err != nil {
		slog.Warn("chmod alert state failed", "err", err)
	}
	if err := os.Rename(tmp.Name(), m.statePath); err != nil {
		_ = os.Remove(tmp.Name())
		slog.Error("save alert state failed", "path", m.statePath, "err", err)
	}
}
