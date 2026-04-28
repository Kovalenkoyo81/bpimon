package notifier

import "fmt"

type Dummy struct{}

func (Dummy) Send(text string) error {
	fmt.Println(text)
	return nil
}

func (Dummy) SendAlert(text string) error {
	fmt.Println("[ALERT] " + text)
	return nil
}
