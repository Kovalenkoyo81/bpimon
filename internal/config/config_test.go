package config

import (
	"slices"
	"strings"
	"testing"
)

// validConfig returns a config that passes Validate; tests break one field at a time.
func validConfig() Config {
	var c Config
	c.Thresholds = Thresholds{IntervalMin: 1, CooldownMin: 30, SmartTemp: 55, SmartLife: 10}
	c.Telegram.Enabled = true
	c.Telegram.Token = "token"
	c.Telegram.ChatID = -100
	c.Devices.Smart = []string{"/dev/sda", "/dev/nvme0n1"}
	c.Devices.MMC = []string{"mmcblk0"}
	c.Devices.Docker = []string{"uptime-kuma", "bpi-auto-discover"}
	return c
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr string // empty — config must be valid
	}{
		{"valid", func(c *Config) {}, ""},
		{"zero interval", func(c *Config) { c.Thresholds.IntervalMin = 0 }, "interval_min"},
		{"zero cooldown", func(c *Config) { c.Thresholds.CooldownMin = 0 }, "cooldown_min"},
		{"smart path outside /dev", func(c *Config) { c.Devices.Smart = []string{"/etc/passwd"} }, "devices.smart"},
		{"smart path with shell chars", func(c *Config) { c.Devices.Smart = []string{"/dev/sda;reboot"} }, "devices.smart"},
		{"smart path traversal", func(c *Config) { c.Devices.Smart = []string{"/dev/../etc/shadow"} }, "devices.smart"},
		{"mmc full path", func(c *Config) { c.Devices.MMC = []string{"/dev/mmcblk0"} }, "devices.mmc"},
		{"mmc partition", func(c *Config) { c.Devices.MMC = []string{"mmcblk0p1"} }, "devices.mmc"},
		{"docker name with space", func(c *Config) { c.Devices.Docker = []string{"bad name"} }, "devices.docker"},
		{"docker name leading dash", func(c *Config) { c.Devices.Docker = []string{"-rm"} }, "devices.docker"},
		{"smart temp required with smart devices", func(c *Config) { c.Thresholds.SmartTemp = 0 }, "smart_temp"},
		{"smart life above 100", func(c *Config) { c.Thresholds.SmartLife = 101 }, "smart_life"},
		{"smart thresholds ignored without smart devices", func(c *Config) {
			c.Devices.Smart = nil
			c.Thresholds.SmartTemp = 0
		}, ""},
		{"telegram enabled without token", func(c *Config) { c.Telegram.Token = "" }, "token"},
		{"telegram enabled without chat id", func(c *Config) { c.Telegram.ChatID = 0 }, "chat_id"},
		{"telegram disabled needs no credentials", func(c *Config) {
			c.Telegram.Enabled = false
			c.Telegram.Token = ""
			c.Telegram.ChatID = 0
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.mutate(&c)
			err := c.Validate()

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid config, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateReportsAllErrors(t *testing.T) {
	c := validConfig()
	c.Thresholds.IntervalMin = 0
	c.Telegram.Token = ""

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"interval_min", "token"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("BPIMON_TELEGRAM_TOKEN", "env-token")
	t.Setenv("BPIMON_TELEGRAM_CHATID", "-200")
	t.Setenv("BPIMON_TELEGRAM_ADMINS", "111, 222,bad,333")

	c := validConfig()
	c.Telegram.Admins = nil
	c.applyEnvOverrides()

	if c.Telegram.Token != "env-token" {
		t.Errorf("token: got %q, want %q", c.Telegram.Token, "env-token")
	}
	if c.Telegram.ChatID != -200 {
		t.Errorf("chat_id: got %d, want -200", c.Telegram.ChatID)
	}
	if want := []int64{111, 222, 333}; !slices.Equal(c.Telegram.Admins, want) {
		t.Errorf("admins: got %v, want %v (invalid entries skipped)", c.Telegram.Admins, want)
	}
}

func TestApplyEnvOverridesInvalidChatIDKeepsFileValue(t *testing.T) {
	t.Setenv("BPIMON_TELEGRAM_CHATID", "not-a-number")

	c := validConfig()
	c.applyEnvOverrides()

	if c.Telegram.ChatID != -100 {
		t.Errorf("chat_id: got %d, want file value -100", c.Telegram.ChatID)
	}
}

func TestApplyEnvOverridesSkippedWhenTelegramDisabled(t *testing.T) {
	t.Setenv("BPIMON_TELEGRAM_TOKEN", "env-token")

	c := validConfig()
	c.Telegram.Enabled = false
	c.applyEnvOverrides()

	if c.Telegram.Token != "token" {
		t.Errorf("token: got %q, want unchanged %q", c.Telegram.Token, "token")
	}
}
