package monitor

type CPUReader interface {
	Usage() (CPUStat, error)
}

// CPUSampler measures CPU usage on demand, over its own window.
type CPUSampler interface {
	Sample() (CPUStat, error)
}

type CPUTempReader interface {
	TempC() (float64, error)
}

type MemoryReader interface {
	Usage() (MemoryStat, error)
}

type DiskReader interface {
	Usage() ([]DiskUsage, error)
}

type SmartReader interface {
	Health() (SmartHealth, error)
	DeviceName() string
}

type SDReader interface {
	Health() (SDHealth, error)
	Name() string
}

type DockerReader interface {
	Health() (DockerHealth, error)
	ContainerName() string
}

type DockerDaemonReader interface {
	Ping() (string, error)
}

// Provider is a metric source. It exposes data through the reader
// interfaces above; rendering is up to the caller.
type Provider interface {
	Name() string
	Kind() Kind
}

// Availabler providers declare whether they can run on the current system.
// Factory skips providers where Available() returns false.
type Availabler interface {
	Available() bool
}
