package monitor

type CPUReader interface {
	Usage() (CPUStat, error)
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


// Provider is the bot/UI interface for all metric sources.
type Provider interface {
	Name() string
	Status() (string, error)
}

// Availabler providers declare whether they can run on the current system.
// Factory skips providers where Available() returns false.
type Availabler interface {
	Available() bool
}
