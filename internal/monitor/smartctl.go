package monitor

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// ssdLifeAttrs — known SSD wear attribute IDs in priority order.
// All use the normalized value field (0-100, higher = more life remaining).
//
//	231 — Kingston, Patriot, Silicon Power, ADATA, SK Hynix, Team Group (SSD_Life_Left)
//	233 — Intel, WD, SanDisk, Corsair, Transcend, Seagate, Toshiba (Media_Wearout_Indicator)
//	177 — Samsung, PNY, MyDigitalSSD (Wear_Leveling_Count)
//	202 — Crucial, Micron, Lexar, Plextor (Percent_Lifetime_Remaining)
//	173 — Toshiba, Kioxia, Phison-based (Wear_Leveling_Count)
//	169 — Apple, OWC (Available_Reservd_Space)
//	210 — Plextor (Unknown_Attribute, % remaining on some models)
var ssdLifeAttrs = []int{231, 233, 177, 202, 173, 169, 210}

func (s Smart) Available() bool {
	_, err := exec.LookPath("smartctl")
	return err == nil
}

func runSmartctl(device string) (SmartHealth, error) {
	out, err := exec.Command("smartctl", "-a", "-j", "--nocheck=standby", device).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Bit 1 (value 2): device in standby — partial output may still be valid
			if exitErr.ExitCode()&2 != 0 && len(out) == 0 {
				return SmartHealth{}, fmt.Errorf("device in standby")
			}
			if len(out) == 0 {
				return SmartHealth{}, fmt.Errorf("smartctl: %w", err)
			}
		} else if len(out) == 0 {
			return SmartHealth{}, fmt.Errorf("smartctl: %w", err)
		}
	}
	return parseSmartctlOutput(out)
}

func parseSmartctlOutput(data []byte) (SmartHealth, error) {
	var raw struct {
		SmartStatus struct {
			Passed bool `json:"passed"`
		} `json:"smart_status"`
		RotationRate *int `json:"rotation_rate"`
		Temperature  *struct {
			Current int `json:"current"`
		} `json:"temperature"`
		AtaSmartAttributes *struct {
			Table []struct {
				ID    int `json:"id"`
				Value int `json:"value"`
				Raw   struct {
					Value int64 `json:"value"`
				} `json:"raw"`
			} `json:"table"`
		} `json:"ata_smart_attributes"`
		NvmeSmartHealth *struct {
			PercentageUsed int `json:"percentage_used"`
		} `json:"nvme_smart_health_information_log"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return SmartHealth{}, fmt.Errorf("parse smartctl output: %w", err)
	}

	h := SmartHealth{
		Health:        "FAILED",
		DiskType:      "Unknown",
		Temp:          -1,
		Life:          -1,
		Reallocated:   -1,
		Pending:       -1,
		Uncorrectable: -1,
	}

	if raw.SmartStatus.Passed {
		h.Health = "PASSED"
	}
	if raw.Temperature != nil {
		h.Temp = raw.Temperature.Current
	}

	switch {
	case raw.NvmeSmartHealth != nil:
		h.DiskType = "NVMe"
		h.Life = 100 - raw.NvmeSmartHealth.PercentageUsed

	case raw.RotationRate != nil && raw.AtaSmartAttributes != nil:
		normByID := make(map[int]int, len(raw.AtaSmartAttributes.Table))
		rawByID := make(map[int]int64, len(raw.AtaSmartAttributes.Table))
		for _, a := range raw.AtaSmartAttributes.Table {
			normByID[a.ID] = a.Value
			rawByID[a.ID] = a.Raw.Value
		}

		if *raw.RotationRate == 0 {
			h.DiskType = "SSD"
			for _, id := range ssdLifeAttrs {
				if v, ok := normByID[id]; ok {
					h.Life = v
					break
				}
			}
		} else {
			h.DiskType = "HDD"
			// attr 5=Reallocated_Sector_Ct, 197=Current_Pending_Sector, 198=Offline_Uncorrectable
			if v, ok := rawByID[5]; ok {
				h.Reallocated = int(v)
			}
			if v, ok := rawByID[197]; ok {
				h.Pending = int(v)
			}
			if v, ok := rawByID[198]; ok {
				h.Uncorrectable = int(v)
			}
		}
	}

	return h, nil
}
