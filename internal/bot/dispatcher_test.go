package bot

import (
	"context"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"bpimon/internal/alert"
)

type fakeAPI struct {
	mu       sync.Mutex
	sent     []string
	answered map[string]string
	edited   map[int]string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{answered: map[string]string{}, edited: map[int]string{}}
}

func (f *fakeAPI) Send(text string) error {
	f.mu.Lock()
	f.sent = append(f.sent, text)
	f.mu.Unlock()
	return nil
}

func (f *fakeAPI) AnswerCallback(id, text string) error {
	f.mu.Lock()
	f.answered[id] = text
	f.mu.Unlock()
	return nil
}

func (f *fakeAPI) EditText(id int, text string) error {
	f.mu.Lock()
	f.edited[id] = text
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
	if !strings.Contains(api.edited[42], "🚨 alert") || !strings.Contains(api.edited[42], "@yuri") {
		t.Fatalf("message not edited with note: %q", api.edited[42])
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
	if len(api.edited) != 0 {
		t.Fatalf("no edit expected: %v", api.edited)
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
