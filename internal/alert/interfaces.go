package alert

type Alert interface {
	Name() string
	Check() (bool, string)
}

// Askable is implemented by alerts whose cause is ambiguous: the condition
// may be intentional (a container stopped by hand) or a real incident.
// The manager attaches two buttons to the alert and keeps asking every
// cooldown until someone answers or the condition clears.
type Askable interface {
	// Choices returns button labels for the two possible answers.
	Choices() (intentional, incident string)
}

// Verdict is the outcome of an Evaluator check.
type Verdict int

const (
	Clear Verdict = iota
	Firing
	// Unknown: the condition cannot be determined right now (e.g. the docker
	// daemon is down). The manager keeps the alert's previous state.
	Unknown
)

// Evaluator is implemented by alerts that can report Unknown. The manager
// prefers Evaluate over Check when both are present.
type Evaluator interface {
	Evaluate() (Verdict, string)
}

func evaluate(a Alert) (Verdict, string) {
	if e, ok := a.(Evaluator); ok {
		return e.Evaluate()
	}
	if ok, msg := a.Check(); ok {
		return Firing, msg
	}
	return Clear, ""
}
