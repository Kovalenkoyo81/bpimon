package alert

type Alert interface {
	Name() string
	Check() (bool, string)
}
