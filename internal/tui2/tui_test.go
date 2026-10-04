package tui2

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/gdamore/tcell/v2"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tui"
	"github.com/revrost/go-openrouter"
)

const waitTimeout = 5 * time.Second

// fakeLLM запоминает, из каких горутин вызывали Usage.
type fakeLLM struct {
	mu         sync.Mutex
	usage      openrouter.Usage
	goroutines []int64
}

func (f *fakeLLM) Usage() openrouter.Usage {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.goroutines = append(f.goroutines, goid())
	return f.usage
}

func (f *fakeLLM) CreateChatCompletion(context.Context, openrouter.ChatCompletionRequest, iface.UI) (*openrouter.ChatCompletionResponse, error) {
	panic("not used")
}

func (f *fakeLLM) calls() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int64(nil), f.goroutines...)
}

func goid() int64 {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	f := strings.Fields(strings.TrimPrefix(string(buf), "goroutine "))
	id, _ := strconv.ParseInt(f[0], 10, 64)
	return id
}

type harness struct {
	t      *testing.T
	ui     *UI
	screen tcell.SimulationScreen
	llm    *fakeLLM
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	llm := &fakeLLM{usage: openrouter.Usage{TotalTokens: 1536, Cost: 1.5}}
	s := tcell.NewSimulationScreen("UTF-8")
	u := newWithScreen(llm, true, s)
	h := &harness{t: t, ui: u, screen: s, llm: llm}
	t.Cleanup(func() { closeWithTimeout(t, u) })
	return h
}

func closeWithTimeout(t *testing.T, u *UI) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		u.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(waitTimeout):
		t.Fatal("Close hangs")
	}
}

// waitFor опрашивает условие до таймаута.
func (h *harness) waitFor(what string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			h.t.Fatalf("timeout waiting for %s; screen:\n%s", what, h.screenText())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (h *harness) state(fn func(u *UI)) {
	h.ui.mu.Lock()
	defer h.ui.mu.Unlock()
	fn(h.ui)
}

func (h *harness) promptWaiting() bool {
	ok := false
	h.state(func(u *UI) { ok = u.promptCh != nil })
	return ok
}

func (h *harness) askWaiting() bool {
	ok := false
	h.state(func(u *UI) { ok = u.ask != nil })
	return ok
}

func (h *harness) screenText() string {
	cells, w, ht := h.screen.GetContents()
	var b strings.Builder
	for y := 0; y < ht; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteByte(' ')
				continue
			}
			b.WriteString(string(c.Runes))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (h *harness) waitScreen(sub string) {
	h.t.Helper()
	h.waitFor("screen contains "+strconv.Quote(sub), func() bool {
		h.ui.redraw()
		return strings.Contains(h.screenText(), sub)
	})
}

func (h *harness) chatTexts() []string {
	var out []string
	h.state(func(u *UI) {
		for _, e := range u.chat {
			out = append(out, e.text)
		}
	})
	return out
}

func (h *harness) waitChat(sub string) {
	h.t.Helper()
	h.waitFor("chat contains "+strconv.Quote(sub), func() bool {
		for _, s := range h.chatTexts() {
			if strings.Contains(s, sub) {
				return true
			}
		}
		return false
	})
}

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
}

func (h *harness) key(k tcell.Key, mod tcell.ModMask) {
	h.screen.InjectKey(k, 0, mod)
}

// prompt запускает Prompt и ждёт, пока он начнёт ждать ввод.
func (h *harness) prompt() <-chan string {
	h.t.Helper()
	ch := make(chan string, 1)
	go func() { ch <- h.ui.Prompt() }()
	h.waitFor("Prompt waiting", h.promptWaiting)
	return ch
}

func (h *harness) ask(question string, options []string) <-chan int {
	h.t.Helper()
	ch := make(chan int, 1)
	go func() { ch <- h.ui.Ask(question, options) }()
	h.waitFor("Ask waiting", h.askWaiting)
	return ch
}

func recvString(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(waitTimeout):
		t.Fatal("timeout waiting for Prompt result")
		return ""
	}
}

func recvInt(t *testing.T, ch <-chan int) int {
	t.Helper()
	select {
	case i := <-ch:
		return i
	case <-time.After(waitTimeout):
		t.Fatal("timeout waiting for Ask result")
		return -1
	}
}

// ---------------------------------------------------------------------------

func TestPromptReturnsText(t *testing.T) {
	h := newHarness(t)
	ch := h.prompt()
	h.typeText("привет, агент")
	h.key(tcell.KeyEnter, tcell.ModAlt)
	h.typeText("вторая строка")
	h.key(tcell.KeyEnter, 0)
	if got := recvString(t, ch); got != "привет, агент\nвторая строка" {
		t.Fatalf("Prompt = %q", got)
	}
	h.waitChat("привет, агент")
	h.state(func(u *UI) {
		if !u.input.isEmpty() {
			t.Errorf("input not cleared: %q", u.input.text())
		}
		if u.baseStatus != statusWorking {
			t.Errorf("status = %q", u.baseStatus)
		}
	})
	h.waitScreen("> привет, агент")
	h.waitScreen(statusWorking)
}

func TestPromptIgnoresEmptyEnter(t *testing.T) {
	h := newHarness(t)
	ch := h.prompt()
	h.key(tcell.KeyEnter, 0)
	h.typeText("   ")
	h.key(tcell.KeyEnter, 0)
	h.key(tcell.KeyEnter, tcell.ModAlt)
	h.key(tcell.KeyEnter, 0)
	// Барьер: событие после Enter обработано, значит и Enter обработан.
	h.typeText("x")
	h.waitFor("x typed", func() bool {
		ok := false
		h.state(func(u *UI) { ok = strings.Contains(u.input.text(), "x") })
		return ok
	})
	select {
	case s := <-ch:
		t.Fatalf("empty Enter submitted %q", s)
	default:
	}
	if !h.promptWaiting() {
		t.Fatal("Prompt must still wait")
	}
	h.key(tcell.KeyEnter, 0)
	if got := recvString(t, ch); got != "x" {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestPromptTypedWhileAgentWorks(t *testing.T) {
	h := newHarness(t)
	h.typeText("early")
	h.key(tcell.KeyEnter, 0) // Prompt не ждёт — Beep, текст остаётся.
	h.waitFor("early typed", func() bool {
		s := ""
		h.state(func(u *UI) { s = u.input.text() })
		return s == "early"
	})
	ch := h.prompt()
	h.key(tcell.KeyEnter, 0)
	if got := recvString(t, ch); got != "early" {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestPromptExitCommands(t *testing.T) {
	for _, cmd := range []string{"/exit", "/quit", "  /exit  "} {
		t.Run(cmd, func(t *testing.T) {
			h := newHarness(t)
			ch := h.prompt()
			h.typeText(cmd)
			h.key(tcell.KeyEnter, 0)
			if got := recvString(t, ch); got != "" {
				t.Fatalf("Prompt = %q, want empty", got)
			}
		})
	}
}

func TestPromptCtrlC(t *testing.T) {
	h := newHarness(t)
	ch := h.prompt()
	h.typeText("unfinished")
	h.key(tcell.KeyCtrlC, tcell.ModCtrl)
	if got := recvString(t, ch); got != "" {
		t.Fatalf("Prompt = %q, want empty", got)
	}
	// Следующий Prompt сразу возвращает "".
	if got := recvString(t, h.promptAsync()); got != "" {
		t.Fatalf("second Prompt = %q", got)
	}
}

func TestPromptCtrlDOnEmpty(t *testing.T) {
	h := newHarness(t)
	ch := h.prompt()
	h.key(tcell.KeyCtrlD, tcell.ModCtrl)
	if got := recvString(t, ch); got != "" {
		t.Fatalf("Prompt = %q, want empty", got)
	}
}

func (h *harness) promptAsync() <-chan string {
	ch := make(chan string, 1)
	go func() { ch <- h.ui.Prompt() }()
	return ch
}

func TestCtrlCWhileAgentWorks(t *testing.T) {
	h := newHarness(t)
	h.key(tcell.KeyCtrlC, tcell.ModCtrl)
	h.waitScreen("Выход после завершения запроса...")
	if got := recvString(t, h.promptAsync()); got != "" {
		t.Fatalf("Prompt after Ctrl+C = %q", got)
	}
}

func TestCommandsDoNotReachAgent(t *testing.T) {
	h := newHarness(t)
	h.ui.SetPlan(&iface.Plan{Items: []iface.PlanItem{{Index: 1, Title: "Сделать раз", Done: true}, {Index: 2, Title: "Сделать два"}}})
	h.ui.SetMemory(&iface.Memory{Items: []iface.MemoryItem{{Key: "ключ", Value: "значение\nмногострочное"}}})
	ch := h.prompt()

	h.typeText("/plan")
	h.key(tcell.KeyEnter, 0)
	h.waitChat("PLAN (1/2)")
	h.waitChat("Сделать два")

	h.typeText("/memory")
	h.key(tcell.KeyEnter, 0)
	h.waitChat("MEMORY (1)")
	h.waitChat("значение\nмногострочное")

	h.typeText("/usage")
	h.key(tcell.KeyEnter, 0)
	h.waitChat("USAGE: 1.5K tokens, $1.50")

	h.typeText("/foo bar")
	h.key(tcell.KeyEnter, 0)
	h.waitChat("Invalid command: \"/foo bar\"")

	select {
	case s := <-ch:
		t.Fatalf("command reached agent: %q", s)
	default:
	}
	if !h.promptWaiting() {
		t.Fatal("Prompt must still wait after commands")
	}
	h.state(func(u *UI) {
		if !u.input.isEmpty() {
			t.Errorf("input not cleared after command: %q", u.input.text())
		}
	})
	h.waitScreen("Invalid command")

	h.typeText("real")
	h.key(tcell.KeyEnter, 0)
	if got := recvString(t, ch); got != "real" {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestPrintPlan(t *testing.T) {
	h := newHarness(t)
	h.ui.SetPlan(&iface.Plan{Items: []iface.PlanItem{{Index: 1, Title: "пункт"}}})
	h.ui.PrintPlan()
	h.waitChat("PLAN (0/1)")
	h.waitChat("пункт")
}

func TestAskArrowsAndEnter(t *testing.T) {
	h := newHarness(t)
	ch := h.ask("Разрешить команду?", []string{"Yes", "No", "Always"})
	h.waitScreen("<Yes>")
	h.waitScreen("Разрешить команду?")
	h.key(tcell.KeyRight, 0)
	h.key(tcell.KeyRight, 0)
	h.key(tcell.KeyRight, 0) // кламп, без зацикливания
	h.waitScreen("<Always>")
	h.key(tcell.KeyLeft, 0)
	h.waitScreen("<No>")
	h.key(tcell.KeyEnter, 0)
	if got := recvInt(t, ch); got != 1 {
		t.Fatalf("Ask = %d, want 1", got)
	}
	h.waitChat("→ No")
	if h.askWaiting() {
		t.Fatal("ask must be reset")
	}
}

func TestAskLeftClamp(t *testing.T) {
	h := newHarness(t)
	ch := h.ask("q", []string{"a", "b"})
	h.key(tcell.KeyLeft, 0)
	h.key(tcell.KeyBacktab, 0)
	h.key(tcell.KeyEnter, 0)
	if got := recvInt(t, ch); got != 0 {
		t.Fatalf("Ask = %d, want 0", got)
	}
}

func TestAskDigits(t *testing.T) {
	h := newHarness(t)
	ch := h.ask("Выберите", []string{"one", "two", "three"})
	h.typeText("9") // вне диапазона — игнорируется
	h.typeText("0")
	h.typeText("x")
	h.typeText("3")
	if got := recvInt(t, ch); got != 2 {
		t.Fatalf("Ask = %d, want 2", got)
	}
	// Текст, набранный во время Ask, не попал в поле ввода.
	h.state(func(u *UI) {
		if !u.input.isEmpty() {
			t.Errorf("input = %q", u.input.text())
		}
	})
}

func TestAskManyEnters(t *testing.T) {
	h := newHarness(t)
	ch := h.ask("q", []string{"a", "b"})
	h.key(tcell.KeyEnter, 0)
	h.key(tcell.KeyEnter, 0)
	h.typeText("2")
	if got := recvInt(t, ch); got != 0 {
		t.Fatalf("Ask = %d, want 0", got)
	}
	// Второй Ask работает, лишние нажатия первого на него не повлияли.
	ch = h.ask("q2", []string{"a", "b"})
	h.typeText("2")
	if got := recvInt(t, ch); got != 1 {
		t.Fatalf("Ask = %d, want 1", got)
	}
}

func TestAskCtrlC(t *testing.T) {
	h := newHarness(t)
	ch := h.ask("Удалить всё?", []string{"Yes", "Allow all", "Deny"})
	h.key(tcell.KeyCtrlC, tcell.ModCtrl)
	if got := recvInt(t, ch); got != 2 {
		t.Fatalf("Ask = %d, want safe choice 2", got)
	}
	if got := recvString(t, h.promptAsync()); got != "" {
		t.Fatalf("Prompt after Ctrl+C in Ask = %q", got)
	}
}

func TestAskEdgeCases(t *testing.T) {
	h := newHarness(t)
	if got := h.ui.Ask("q", nil); got != 0 {
		t.Fatalf("Ask(nil) = %d", got)
	}
	ch := h.ask("q", []string{"only"})
	h.key(tcell.KeyRight, 0)
	h.key(tcell.KeyEnter, 0)
	if got := recvInt(t, ch); got != 0 {
		t.Fatalf("Ask = %d", got)
	}
	// Ask, прерванный Close, возвращает индекс в диапазоне.
	ch = h.ask("q", []string{"Yes", "No"})
	closeWithTimeout(t, h.ui)
	if got := recvInt(t, ch); got != 1 {
		t.Fatalf("Ask after Close = %d, want 1", got)
	}
	if got := h.ui.Ask("q", []string{"a", "b", "c"}); got < 0 || got > 2 {
		t.Fatalf("Ask on closed UI = %d", got)
	}
}

func TestAskPgUpScrollsChat(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 100; i++ {
		h.ui.Response("agent", "line "+strconv.Itoa(i))
	}
	h.waitScreen("line 99")
	ch := h.ask("q", []string{"a", "b"})
	h.key(tcell.KeyPgUp, 0)
	h.waitFor("chat scrolled", func() bool {
		ok := false
		h.state(func(u *UI) { ok = !u.chatFollow })
		return ok
	})
	h.typeText("1")
	if got := recvInt(t, ch); got != 0 {
		t.Fatalf("Ask = %d", got)
	}
}

func TestSetPlanCopies(t *testing.T) {
	h := newHarness(t)
	p := &iface.Plan{Items: []iface.PlanItem{{Index: 1, Title: "Первый пункт"}, {Index: 2, Title: "Второй пункт", Done: true}}}
	h.ui.SetPlan(p)
	p.Items[0].Title = "ИЗМЕНЕНО"
	p.Items[0].Done = true
	p.Items = append(p.Items, iface.PlanItem{Index: 3, Title: "лишний"})
	h.state(func(u *UI) {
		if len(u.plan.Items) != 2 || u.plan.Items[0].Title != "Первый пункт" || u.plan.Items[0].Done {
			t.Fatalf("plan changed with source: %+v", u.plan)
		}
	})
	h.waitScreen("1) ◻ Первый пункт")
	h.waitScreen("2) ▣ Второй пункт")
	h.waitScreen("План 1/2")
	if strings.Contains(h.screenText(), "ИЗМЕНЕНО") {
		t.Fatal("source mutation leaked to screen")
	}
	h.ui.SetPlan(nil)
	h.waitScreen("План 0/0")
	h.waitScreen("(пусто)")
}

func TestSetMemoryCopies(t *testing.T) {
	h := newHarness(t)
	m := &iface.Memory{Items: []iface.MemoryItem{{Key: "project_goal", Value: "секретное значение"}, {Key: "ключ\x1b[31m", Value: "v"}}}
	h.ui.SetMemory(m)
	m.Items[0].Key = "MUTATED"
	m.Items[0].Value = "MUTATED"
	h.state(func(u *UI) {
		if u.memory.Items[0].Key != "project_goal" || u.memory.Items[0].Value != "секретное значение" {
			t.Fatalf("memory changed with source: %+v", u.memory)
		}
		if u.memory.Items[1].Key != "ключ" {
			t.Fatalf("memory key not sanitized: %q", u.memory.Items[1].Key)
		}
	})
	h.waitScreen("• project_goal")
	h.waitScreen("• ключ")
	h.waitScreen("Память (2)")
	text := h.screenText()
	if strings.Contains(text, "секретное значение") || strings.Contains(text, "MUTATED") {
		t.Fatal("memory panel must show only keys of the copy")
	}
	h.ui.SetMemory(nil)
	h.waitScreen("Память (0)")
}

func TestScreenShowsWorkDirStatusUsage(t *testing.T) {
	h := newHarness(t)
	h.waitScreen(statusIdle)
	h.waitScreen(filepath.Base(h.ui.workDir))
	h.waitScreen("1.5K tokens, $1.50")
	h.waitScreen("[ Ввод ]")
	h.waitScreen("[ Токены ]")
	h.waitScreen("Enter — отправить")

	tok := h.ui.Thinking()
	h.waitScreen("Думаю...")
	tok.Done()
	tok.Done()
	h.waitFor("thinking status removed", func() bool {
		n := 0
		h.state(func(u *UI) { n = len(u.status) })
		return n == 0
	})
	h.waitScreen(statusIdle)

	h.ui.RateLimit(time.Hour)
	h.waitScreen("Лимит запросов")
	h.waitChat("Rate limit exceeded")
}

func TestResizeAndTooSmall(t *testing.T) {
	h := newHarness(t)
	h.screen.SetSize(30, 8)
	h.waitScreen("Окно слишком маленькое")
	h.screen.SetSize(200, 60)
	h.waitScreen(statusIdle)
	h.screen.SetSize(40, 10)
	h.waitScreen("[ Ввод ]")
	ch := h.prompt()
	h.typeText("ok")
	h.key(tcell.KeyEnter, 0)
	if got := recvString(t, ch); got != "ok" {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestUsageNotReadFromRenderGoroutine(t *testing.T) {
	h := newHarness(t)
	me := goid()
	ch := h.prompt()
	h.typeText("/usage")
	h.key(tcell.KeyEnter, 0)
	h.waitChat("USAGE:")
	tok := h.ui.ToolCall(iface.ToolCall{Type: "shell", Args: "ls"})
	// Пара тиков спиннера, чтобы tickLoop точно отработал.
	var frame int
	h.state(func(u *UI) { frame = u.frame })
	h.waitFor("ticks", func() bool {
		f := 0
		h.state(func(u *UI) { f = u.frame })
		return f >= frame+2
	})
	tok.Success()

	h.llm.mu.Lock()
	h.llm.usage = openrouter.Usage{TotalTokens: 5 << 20, Cost: 12.345}
	h.llm.mu.Unlock()
	h.ui.Usage()
	h.waitScreen("5.0M tokens, $12.35")

	calls := h.llm.calls()
	if len(calls) != 2 {
		t.Fatalf("llm.Usage called %d times, want 2 (New + Usage)", len(calls))
	}
	for _, id := range calls {
		if id != me {
			t.Fatalf("llm.Usage called from goroutine %d, test goroutine %d", id, me)
		}
	}
	h.key(tcell.KeyCtrlC, tcell.ModCtrl)
	recvString(t, ch)
}

func TestToolCallSuccessFailure(t *testing.T) {
	h := newHarness(t)
	ok := h.ui.ToolCall(iface.ToolCall{Type: "read_file", Args: "main.go\nвторая строка"})
	h.waitScreen("read_file main.go")
	if strings.Contains(h.screenText(), "вторая строка") {
		t.Fatal("running tool must show only first line of args")
	}
	ok.Success()
	ok.Failure("must be ignored")
	h.waitScreen("✓ read_file main.go")
	h.waitScreen("вторая строка")

	fail := h.ui.ToolCall(iface.ToolCall{Type: "shell", Args: "rm -rf /"})
	fail.Failure("permission\x1b[31m denied")
	fail.Success()
	h.waitScreen("✗ shell rm -rf /")
	h.waitScreen("permission denied")

	h.state(func(u *UI) {
		var kinds []entryKind
		for _, e := range u.chat {
			if e.kind != kindText {
				kinds = append(kinds, e.kind)
			}
		}
		if len(kinds) != 2 || kinds[0] != kindToolOK || kinds[1] != kindToolFail {
			t.Fatalf("kinds = %v", kinds)
		}
		if u.runningTools != 0 || len(u.status) != 0 {
			t.Fatalf("runningTools=%d status=%v", u.runningTools, u.status)
		}
	})
}

func TestResponsesAreSanitized(t *testing.T) {
	h := newHarness(t)
	h.ui.Response("coder", "ответ\x1b]0;evil\x07 \u202eчисто\x1b[2J")
	h.ui.Reasoning("coder", "думаю\tтак")
	h.ui.Refusal("", "отказ")
	h.waitScreen("coder: ответ чисто")
	h.waitScreen("думаю    так")
	h.waitScreen("отказ")
	for _, s := range h.chatTexts() {
		if strings.ContainsAny(s, "\x1b\u202e\x07\t") {
			t.Fatalf("unsanitized chat entry %q", s)
		}
	}
}

func TestPrintfHandler(t *testing.T) {
	var out bytes.Buffer
	old := color.Output
	color.Output = &out
	defer func() { color.Output = old }()

	h := newHarness(t)
	tui.Printf("из песочницы %d\x1b[31m", 42)
	h.waitChat("из песочницы 42")
	h.waitScreen("из песочницы 42")
	if out.Len() != 0 {
		t.Fatalf("Printf leaked to stdout: %q", out.String())
	}

	closeWithTimeout(t, h.ui)
	tui.Printf("после закрытия")
	if !strings.Contains(out.String(), "после закрытия") {
		t.Fatalf("Printf handler not removed in Close: %q", out.String())
	}
	for _, s := range h.chatTexts() {
		if strings.Contains(s, "после закрытия") {
			t.Fatal("Printf after Close reached chat")
		}
	}
}

func TestCloseIdempotent(t *testing.T) {
	h := newHarness(t)
	h.ui.Response("a", "b")
	ch := h.promptAsync()
	h.waitFor("Prompt waiting", h.promptWaiting)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ui.Close()
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(waitTimeout):
		t.Fatal("concurrent Close hangs")
	}
	closeWithTimeout(t, h.ui)

	if got := recvString(t, ch); got != "" {
		t.Fatalf("Prompt after Close = %q", got)
	}
	select {
	case <-h.ui.loopDone:
	case <-time.After(waitTimeout):
		t.Fatal("eventLoop did not exit")
	}

	// После Close всё — no-op и без паник/зависаний.
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		h.ui.Response("a", "b")
		h.ui.Reasoning("a", "b")
		h.ui.Refusal("a", "b")
		h.ui.RateLimit(time.Second)
		h.ui.Thinking().Done()
		tc := h.ui.ToolCall(iface.ToolCall{Type: "x"})
		tc.Success()
		h.ui.SetPlan(&iface.Plan{})
		h.ui.SetMemory(&iface.Memory{})
		h.ui.PrintPlan()
		h.ui.Usage()
		_ = h.ui.Prompt()
	}()
	select {
	case <-finished:
	case <-time.After(waitTimeout):
		t.Fatal("methods after Close hang")
	}
}

func TestCloseWithFullEventQueue(t *testing.T) {
	h := newHarness(t)
	// Забиваем очередь перерисовками и закрываем — Close не должен виснуть.
	for i := 0; i < 50; i++ {
		h.ui.redraw()
	}
	closeWithTimeout(t, h.ui)
}

