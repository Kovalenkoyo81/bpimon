package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Thresholds struct {
	CPU         float64 `yaml:"cpu"`
	CPUTemp     float64 `yaml:"cpu_temp"`
	RAM         int     `yaml:"ram"`
	Disk        int     `yaml:"disk"`
	SmartTemp   int     `yaml:"smart_temp"`
	SmartLife   int     `yaml:"smart_life"`
	IntervalMin int     `yaml:"interval_min"`
	CooldownMin int     `yaml:"cooldown_min"`
}

type Config struct {
	Telegram struct {
		Enabled bool    `yaml:"enabled"`
		Token   string  `yaml:"token"`
		ChatID  int64   `yaml:"chat_id"`
		Admins  []int64 `yaml:"admins"`
	} `yaml:"telegram"`

	Thresholds Thresholds `yaml:"thresholds"`

	Devices struct {
		Smart  []string `yaml:"smart"`
		MMC    []string `yaml:"mmc"`
		Docker []string `yaml:"docker"`
	} `yaml:"devices"`
}

func Load(path string) *Config {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read config file %q: %v\n", path, err)
		os.Exit(1)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse config file %q: %v\n", path, err)
		os.Exit(1)
	}
	c.applyEnvOverrides()
	if err := c.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	return &c
}

func (c *Config) Validate() error {
	var errs []string

	if c.Thresholds.IntervalMin <= 0 {
		errs = append(errs, "thresholds.interval_min must be > 0")
	}
	if c.Thresholds.CooldownMin <= 0 {
		errs = append(errs, "thresholds.cooldown_min must be > 0")
	}
	for _, dev := range c.Devices.Smart {
		if !strings.HasPrefix(dev, "/dev/") {
			errs = append(errs, fmt.Sprintf("devices.smart: %q must start with /dev/", dev))
		}
	}
	for _, dev := range c.Devices.MMC {
		if strings.HasPrefix(dev, "/dev/") {
			errs = append(errs, fmt.Sprintf("devices.mmc: %q must be a block device name (e.g. mmcblk0), not a path", dev))
		}
	}
	if len(c.Devices.Smart) > 0 {
		if c.Thresholds.SmartTemp <= 0 {
			errs = append(errs, "thresholds.smart_temp must be > 0")
		}
		if c.Thresholds.SmartLife < 0 || c.Thresholds.SmartLife > 100 {
			errs = append(errs, "thresholds.smart_life must be between 0 and 100")
		}
	}
	if c.Telegram.Enabled {
		if c.Telegram.Token == "" {
			errs = append(errs, "telegram.enabled is true but token is not set (BPIMON_TELEGRAM_TOKEN)")
		}
		if c.Telegram.ChatID == 0 {
			errs = append(errs, "telegram.enabled is true but chat_id is not set (BPIMON_TELEGRAM_CHATID)")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// applyEnvOverrides loads Telegram credentials from environment variables,
// overriding any values set in the config file.
func (c *Config) applyEnvOverrides() {
	if !c.Telegram.Enabled {
		return
	}
	if token := os.Getenv("BPIMON_TELEGRAM_TOKEN"); token != "" {
		c.Telegram.Token = token
	}
	if raw := os.Getenv("BPIMON_TELEGRAM_CHATID"); raw != "" {
		var id int64
		if _, err := fmt.Sscan(raw, &id); err == nil {
			c.Telegram.ChatID = id
		}
	}
	if raw := os.Getenv("BPIMON_TELEGRAM_ADMINS"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err == nil {
				c.Telegram.Admins = append(c.Telegram.Admins, id)
			}
		}
	}
}
