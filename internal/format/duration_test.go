package format

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	cases := map[time.Duration]string{
		7 * time.Second:               "7s",
		4*time.Minute + 5*time.Second: "4m 5s",
		3*time.Hour + 12*time.Minute:  "3h 12m",
		51*time.Hour + 30*time.Minute: "2d 3h",
		1500 * time.Millisecond:       "2s",
	}
	for d, want := range cases {
		if got := Duration(d); got != want {
			t.Errorf("Duration(%v) = %q, want %q", d, got, want)
		}
	}
}
