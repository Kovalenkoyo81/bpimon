package notifier

import (
	"fmt"

	"github.com/Kovalenkoyo81/bpimon/internal/format"
)

// Dummy prints messages to stdout; used when Telegram is disabled.
type Dummy struct{}

func (Dummy) Send(msg format.HTML) error {
	fmt.Println(msg.String())
	return nil
}
