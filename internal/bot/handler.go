package bot

import (
	"time"

	"bpimon/internal/alert"
	"bpimon/internal/monitor"
	"bpimon/internal/system"
)

// SystemActions performs host actions requested by admins.
type SystemActions interface {
	CanPower() bool
	Reboot() error
	Poweroff() error
	RestartContainer(name string) error
	ContainerState(name string) (system.ContainerState, error)
}

// HandlerConfig lists the Handler's dependencies.
type HandlerConfig struct {
	Providers []monitor.Provider
	Admins    []int64
	Silencer  alert.Silencer   // nil: /silence unavailable
	Version   *VersionReporter // nil: /version unavailable
	System    SystemActions
}

// Handler turns chat commands into responses.
type Handler struct {
	providers []monitor.Provider
	admins    []int64
	silencer  alert.Silencer
	version   *VersionReporter
	system    SystemActions

	confirm *Confirmator
	limiter *RateLimiter
}

func NewHandler(c HandlerConfig) *Handler {
	return &Handler{
		providers: c.Providers,
		admins:    c.Admins,
		silencer:  c.Silencer,
		version:   c.Version,
		system:    c.System,
		confirm:   NewConfirmator(60 * time.Second),
		limiter:   NewRateLimiter(10*time.Second, 5),
	}
}

// IsAdmin reports whether id is listed in Admins.
func (h *Handler) IsAdmin(id int64) bool {
	for _, a := range h.admins {
		if a == id {
			return true
		}
	}
	return false
}
