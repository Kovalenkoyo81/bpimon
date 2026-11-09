package bot

import (
	"context"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/Kovalenkoyo81/bpimon/internal/alert"
	"github.com/Kovalenkoyo81/bpimon/internal/format"
)

type fakeAPI struct {
	mu       sync.Mutex
	sent     []string
	answered map[string]string
	status   map[int]string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{answered: map[string]string{}, status: map[int]string{}}
}

func (f *fakeAPI) Send(msg format.HTML) error {
	f.mu.Lock()
	f.sent = append(f.sent, msg.String())
	f.mu.Unlock()
	return nil
}

func (f *fakeAPI) AnswerCallback(id, text string) error {
	f.mu.Lock()
	f.answered[id] = text
	f.mu.Unlock()
	return nil
}

func (f *fakeAPI) SetStatusButton(id int, text, data string) error {
	f.mu.Lock()
	f.status[id] = text + "|" + data
	f.mu.Unlock()
	return nil
}

const testChat = -100

func newTestDispatcher(res *stubResolver) (*Dispatcher, *fakeAPI) {
	api := newFakeAPI()
	h := newHandler([]int64{1}, nil)
	cb := &CallbackHandler{Resolver: res, IsAdmin: h.IsAdmin}
	return NewDispatcher(api, h, cb, testChat), api
}

func msg(chat int64, from int64, text string) tgbotapi.Update {
	return tgbotapi.Update{Message: &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chat}, From: &tgbotapi.User{ID: from}, Text: text,
	}}
}

func press(chat int64, from int64, data string) tgbotapi.Update {
	return tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		ID:      "cb1",
		From:    &tgbotapi.User{ID: from, UserName: "yuri"},
		Data:    data,
		Message: &tgbotapi.Message{MessageID: 42, Chat: &tgbotapi.Chat{ID: chat}, Text: "🚨 alert"},
	}}
}

func TestDispatcherRoutesMessagesFromConfiguredChatOnly(t *testing.T) {
	d, api := newTestDispatcher(&stubResolver{})
	d.dispatch(msg(testChat, 1, "/help"))
	d.dispatch(msg(12345, 1, "/help")) // other chat: ignored
	if len(api.sent) != 1 || !strings.Contains(api.sent[0], "/status") {
		t.Fatalf("expected one menu reply, got %v", api.sent)
	}
}

func TestDispatcherCallbackAnswersAndEdits(t *testing.T) {
	res := &stubResolver{}
	d, api := newTestDispatcher(res)
	d.dispatch(press(testChat, 1, alert.CallbackData("Docker app", alert.Intentional)))

	if res.name != "Docker app" || res.r != alert.Intentional {
		t.Fatalf("resolver not called: %+v", res)
	}
	if _, ok := api.answered["cb1"]; !ok {
		t.Fatal("callback must always be answered")
	}
	if !strings.Contains(api.status[42], "Stopped by admin · @yuri") || !strings.HasSuffix(api.status[42], "|"+AnsweredData) {
		t.Fatalf("buttons not replaced by status: %q", api.status[42])
	}
}

func TestDispatcherCallbackFromOtherChatIsAnsweredButIgnored(t *testing.T) {
	res := &stubResolver{}
	d, api := newTestDispatcher(res)
	d.dispatch(press(999, 1, alert.CallbackData("Docker app", alert.Intentional)))
	if res.name != "" {
		t.Fatal("resolver must not be called for another chat")
	}
	if _, ok := api.answered["cb1"]; !ok {
		t.Fatal("callback must be answered even when ignored, or the button keeps spinning")
	}
	if len(api.status) != 0 {
		t.Fatalf("no status change expected: %v", api.status)
	}
}

func TestDispatcherRunStopsOnCloseAndCancel(t *testing.T) {
	d, _ := newTestDispatcher(&stubResolver{})
	ch := make(chan tgbotapi.Update)
	done := make(chan struct{})
	go func() { d.Run(context.Background(), ch); close(done) }()
	close(ch)
	<-done

	ctx, cancel := context.WithCancel(context.Background())
	done = make(chan struct{})
	go func() { d.Run(ctx, make(chan tgbotapi.Update)); close(done) }()
	cancel()
	<-done
}

func TestDispatcherStatusButtonAnswersAlreadyAnswered(t *testing.T) {
	res := &stubResolver{}
	d, api := newTestDispatcher(res)
	d.dispatch(press(testChat, 1, AnsweredData))
	if api.answered["cb1"] != "Already answered" || res.name != "" {
		t.Fatalf("answered=%q resolver=%+v", api.answered["cb1"], res)
	}
}
