package monitor

import "testing"

var ssdFixture = []byte(`{
  "smart_status": {"passed": true},
  "rotation_rate": 0,
  "temperature": {"current": 41},
  "ata_smart_attributes": {
    "table": [
      {"id": 231, "value": 86, "raw": {"value": 0}}
    ]
  }
}`)

var hddFixture = []byte(`{
  "smart_status": {"passed": true},
  "rotation_rate": 7200,
  "temperature": {"current": 35},
  "ata_smart_attributes": {
    "table": [
      {"id": 5,   "value": 100, "raw": {"value": 0}},
      {"id": 197, "value": 100, "raw": {"value": 2}},
      {"id": 198, "value": 100, "raw": {"value": 1}}
    ]
  }
}`)

var nvmeFixture = []byte(`{
  "smart_status": {"passed": true},
  "temperature": {"current": 38},
  "nvme_smart_health_information_log": {"percentage_used": 12}
}`)

var failedFixture = []byte(`{
  "smart_status": {"passed": false},
  "rotation_rate": 0,
  "temperature": {"current": 55},
  "ata_smart_attributes": {"table": []}
}`)

func TestParseSmartSSD(t *testing.T) {
	h, err := parseSmartctlOutput(ssdFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Health != "PASSED" {
		t.Errorf("health: got %q want PASSED", h.Health)
	}
	if h.DiskType != "SSD" {
		t.Errorf("diskType: got %q want SSD", h.DiskType)
	}
	if h.Temp != 41 {
		t.Errorf("temp: got %d want 41", h.Temp)
	}
	if h.Life != 86 {
		t.Errorf("life: got %d want 86", h.Life)
	}
}

func TestParseSmartHDD(t *testing.T) {
	h, err := parseSmartctlOutput(hddFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.DiskType != "HDD" {
		t.Errorf("diskType: got %q want HDD", h.DiskType)
	}
	if h.Reallocated != 0 {
		t.Errorf("reallocated: got %d want 0", h.Reallocated)
	}
	if h.Pending != 2 {
		t.Errorf("pending: got %d want 2", h.Pending)
	}
	if h.Uncorrectable != 1 {
		t.Errorf("uncorrectable: got %d want 1", h.Uncorrectable)
	}
}

func TestParseSmartNVMe(t *testing.T) {
	h, err := parseSmartctlOutput(nvmeFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.DiskType != "NVMe" {
		t.Errorf("diskType: got %q want NVMe", h.DiskType)
	}
	if h.Life != 88 {
		t.Errorf("life: got %d want 88 (100-12)", h.Life)
	}
}

func TestParseSmartFailed(t *testing.T) {
	h, err := parseSmartctlOutput(failedFixture)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Health != "FAILED" {
		t.Errorf("health: got %q want FAILED", h.Health)
	}
}

func TestParseSmartInvalidJSON(t *testing.T) {
	_, err := parseSmartctlOutput([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
