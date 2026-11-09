package monitor

// Kind identifies what a provider measures. Commands select providers by
// Kind rather than by display name, so renaming a provider cannot silently
// break a command.
type Kind int

const (
	KindCPU Kind = iota + 1
	KindRAM
	KindDisk
	KindSmart
	KindSD
	KindDocker // containers and the docker daemon
)

func (*CPU) Kind() Kind         { return KindCPU }
func (Memory) Kind() Kind       { return KindRAM }
func (Disk) Kind() Kind         { return KindDisk }
func (Smart) Kind() Kind        { return KindSmart }
func (SD) Kind() Kind           { return KindSD }
func (Docker) Kind() Kind       { return KindDocker }
func (DockerDaemon) Kind() Kind { return KindDocker }
