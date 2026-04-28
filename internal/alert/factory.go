package alert

import (
	"bpimon/internal/config"
	"bpimon/internal/monitor"
)

// FromProviders creates alerts for all given providers.
// Providers with no matching alert type are silently skipped.
func FromProviders(providers []monitor.Provider, t config.Thresholds) []Alert {
	var alerts []Alert
	for _, p := range providers {
		switch v := p.(type) {
		case *monitor.CPU:
			alerts = append(alerts, NewCPU(v, t.CPU))
			alerts = append(alerts, NewCPUTemp(v, t.CPUTemp))
		case monitor.Memory:
			alerts = append(alerts, NewMemory(v, t.RAM))
		case monitor.Disk:
			alerts = append(alerts, NewDisk(v, t.Disk))
		case monitor.Smart:
			alerts = append(alerts, NewSmart(v, t.SmartTemp, t.SmartLife))
		case monitor.SD:
			alerts = append(alerts, NewSD(v))
		case monitor.Docker:
			alerts = append(alerts, NewDocker(v))
		}
	}
	return alerts
}
