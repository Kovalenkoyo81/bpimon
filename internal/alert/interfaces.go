package alert

// Verdict is the outcome of evaluating an alert condition.
type Verdict int

const (
	Clear Verdict = iota
	Firing
	// Unknown: the condition cannot be determined right now (a read
	// failed, the docker daemon is down, ...). The manager keeps the
	// alert's previous state — no false recovery, no false alarm — and
	// logs the reason once.
	Unknown
)

func (v Verdict) String() string {
	switch v {
	case Clear:
		return "clear"
	case Firing:
		return "firing"
	default:
		return "unknown"
	}
}

// Alert is a single monitored condition.
type Alert interface {
	Name() string
	// Evaluate checks the condition. The message describes the problem
	// for Firing, or the reason for Unknown; it is empty for Clear.
	Evaluate() (Verdict, string)
}

// Askable is implemented by alerts whose cause is ambiguous: the condition
// may be intentional (a container stopped by hand) or a real incident.
// The manager attaches two buttons to the alert and keeps asking every
// cooldown until someone answers or the condition clears.
type Askable interface {
	// Choices returns button labels for the two possible answers.
	Choices() (intentional, incident string)
}
