// Package tui2 реализует полноэкранный интерфейс iface.UI на tcell (флаг -modern-ui).
package tui2

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kapitanov/ucode/internal/iface"
	"github.com/kapitanov/ucode/internal/tui"
)

const (
	statusIdle    = "Ожидание ввода"
	statusWorking = "Обработка запроса"
	tickInterval  = 100 * time.Millisecond
	closeTimeout  = time.Second
	tailLines     = 20
)

// UI — полноэкранный интерфейс. Состояние защищено mu; screen трогает
// только eventLoop (а также Init/Fini/PostEvent).
type UI struct {
	screen         tcell.Screen
	llm            iface.LLM
	printReasoning bool
	workDir        string
	exit           func(code int)

	mu               sync.Mutex
	chat             []*chatEntry
	newWhileScrolled bool
	plan             iface.Plan
	memory           iface.Memory
	usageText        string
	status           []statusItem
	baseStatus       string
	nextID           int
	runningTools     int
	focus            panel
	scroll           [panelCount]int
	lines            [panelCount]int
	chatFollow       bool
	input            inputState
	ask              *askState
	promptCh         chan string
	quitRequested    bool
	lastCtrlC        time.Time
	frame            int
	closed           bool
	lay              layout

	done      chan struct{}
	loopDone  chan struct{}
	closeOnce sync.Once
	finiOnce  sync.Once
}

// New создаёт UI на реальном терминале.
func New(llm iface.LLM, printReasoning bool) *UI {
	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}
	return newWithScreen(llm, printReasoning, screen)
}

// newWithScreen — конструктор для подстановки экрана (в т.ч. SimulationScreen).
func newWithScreen(llm iface.LLM, printReasoning bool, screen tcell.Screen) *UI {
	if err := screen.Init(); err != nil {
		panic(err)
	}
	screen.EnableMouse(tcell.MouseButtonEvents)
	screen.EnablePaste()

	wd, _ := os.Getwd()
	u := &UI{
		screen:         screen,
		llm:            llm,
		printReasoning: printReasoning,
		workDir:        sanitize(wd),
		exit:           os.Exit,
		baseStatus:     statusIdle,
		chatFollow:     true,
		focus:          panelInput,
		input:          newInputState(),
		done:           make(chan struct{}),
		loopDone:       make(chan struct{}),
	}

	tui.SetPrintfHandler(u.printf)
	go u.eventLoop()
	go u.tickLoop()
	u.Usage()
	return u
}

// redraw будит eventLoop; переполнение очереди не страшно — перерисовка и так в ней.
func (u *UI) redraw() {
	_ = u.screen.PostEvent(tcell.NewEventInterrupt(nil))
}

// update меняет состояние под mu и просит перерисовку; после Close — no-op.
func (u *UI) update(fn func()) {
	if u.locked(fn) {
		u.redraw()
	}
}

func (u *UI) locked(fn func()) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return false
	}
	fn()
	return true
}

func (u *UI) recoverPanic() {
	if r := recover(); r != nil {
		u.fini()
		tui.SetPrintfHandler(nil)
		_, _ = fmt.Fprintf(os.Stderr, "tui2: panic: %v\n%s", r, debug.Stack())
		panic(r)
	}
}

func (u *UI) fini() {
	u.finiOnce.Do(u.screen.Fini)
}

func (u *UI) eventLoop() {
	defer close(u.loopDone)
	defer u.recoverPanic()

	u.step(nil)
	for {
		ev := u.screen.PollEvent()
		if ev == nil {
			return
		}
		select {
		case <-u.done:
			return
		default:
		}
		if u.step(ev) {
			// Повторный Ctrl+C: аварийный выход, экран восстанавливаем до Exit.
			u.shutdown(false)
			u.exit(130)
			return
		}
	}
}

// step обрабатывает событие и перерисовывает экран; true — аварийный выход.
func (u *UI) step(ev tcell.Event) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if ev != nil && u.handleEvent(ev) {
		return true
	}
	u.draw()
	return false
}

func (u *UI) tickLoop() {
	defer u.recoverPanic()

	t := time.NewTicker(tickInterval)
	defer t.Stop()
	for {
		select {
		case <-u.done:
			return
		case <-t.C:
		}
		active := false
		u.locked(func() {
			active = len(u.status) > 0 || u.runningTools > 0
			if active {
				u.frame++
			}
		})
		if active {
			u.redraw()
		}
	}
}

func (u *UI) printf(msg string) {
	msg = sanitize(msg)
	u.update(func() {
		u.addEntry(newEntry(nil, msg, stylePrintf))
	})
}

func (u *UI) Prompt() string {
	ch := make(chan string, 1)
	ok := false
	u.locked(func() {
		if u.quitRequested {
			return
		}
		if u.promptCh != nil {
			// Уже есть активный Prompt — не перезаписываем канал первого вызова.
			return
		}
		u.promptCh = ch
		u.baseStatus = statusIdle
		ok = true
	})
	if !ok {
		return ""
	}
	u.redraw()

	var s string
	select {
	case s = <-ch:
	case <-u.done:
	}

	u.update(func() {
		u.promptCh = nil
		u.baseStatus = statusWorking
	})
	return s
}

func (u *UI) Ask(question string, options []string) int {
	if len(options) == 0 {
		return 0
	}
	question = sanitize(question)
	opts := make([]string, len(options))
	for i, o := range options {
		opts[i] = sanitize(o)
	}

	a := &askState{question: question, options: opts, ch: make(chan int, 1)}
	if !u.locked(func() {
		u.addEntry(newEntry(nil, question, styleQuestion))
		u.ask = a
		u.focus = panelInput
	}) {
		return safeChoice(opts)
	}
	u.redraw()

	var i int
	select {
	case i = <-a.ch:
	case <-u.done:
		i = safeChoice(opts)
	}
	i = min(max(i, 0), len(opts)-1)

	u.update(func() {
		u.ask = nil
		u.addEntry(newEntry(nil, "→ "+opts[i], styleQuestion))
	})
	return i
}

func (u *UI) Thinking() iface.ThinkingToken {
	t := &thinkingToken{u: u}
	u.update(func() {
		t.id = u.pushStatus("Думаю...", time.Time{})
	})
	return t
}

type thinkingToken struct {
	u    *UI
	id   int
	once sync.Once
}

func (t *thinkingToken) Done() {
	t.once.Do(func() {
		t.u.update(func() { t.u.popStatus(t.id) })
	})
}

func (u *UI) RateLimit(waitDuration time.Duration) {
	u.update(func() {
		u.addEntry(newEntry(nil, fmt.Sprintf("Rate limit exceeded. Waiting for %s...", waitDuration), styleRed))
		u.pushStatus(fmt.Sprintf("Лимит запросов, ждём %s", waitDuration), time.Now().Add(waitDuration))
	})
}

func (u *UI) Reasoning(agentName, response string) {
	if !u.printReasoning {
		return
	}
	u.agentText(agentName, response, styleReasoning.Bold(true), styleReasoning)
}

func (u *UI) Response(agentName, response string) {
	u.agentText(agentName, response, styleBold, styleWhite)
}

func (u *UI) Refusal(agentName, response string) {
	u.agentText(agentName, response, styleRed.Bold(true), styleRed)
}

func (u *UI) agentText(agentName, text string, nameStyle, textStyle tcell.Style) {
	agentName, text = sanitize(agentName), sanitize(text)
	u.update(func() {
		u.addAgentText(agentName, text, nameStyle, textStyle)
	})
}

func (u *UI) ToolCall(toolCall iface.ToolCall) iface.ToolCallToken {
	typ, args := sanitize(toolCall.Type), sanitize(toolCall.Args)
	short, _, _ := strings.Cut(args, "\n")

	t := &toolCallToken{u: u, typ: typ, args: args}
	t.entry = &chatEntry{kind: kindToolRunning, text: typ + " " + short, style: styleToolRun, prefixW: 2}
	u.update(func() {
		u.addEntry(t.entry)
		t.id = u.pushStatus(typ, time.Time{})
		u.runningTools++
	})
	return t
}

type toolCallToken struct {
	u         *UI
	typ, args string
	entry     *chatEntry
	id        int
	once      sync.Once
}

func (t *toolCallToken) Success() {
	t.finish(kindToolOK, styleToolOK, "")
}

func (t *toolCallToken) Failure(message string) {
	t.finish(kindToolFail, styleRed.Dim(true), sanitize(message))
}

func (t *toolCallToken) finish(kind entryKind, style tcell.Style, detail string) {
	t.once.Do(func() {
		t.u.update(func() {
			e := t.entry
			e.kind, e.style, e.detail = kind, style, detail
			e.text = t.typ + " " + t.args
			e.wrappedW = 0
			t.u.popStatus(t.id)
			t.u.runningTools--
		})
	})
}

// SetPlan хранит копию плана. Ограничение: субагент передаёт свой план,
// и панель показывает последний переданный (план тимлида вернётся при его следующем изменении).
func (u *UI) SetPlan(plan *iface.Plan) {
	p := clonePlan(plan)
	u.update(func() { u.plan = p })
}

func (u *UI) PrintPlan() {
	u.update(u.addPlanToChat)
}

func (u *UI) SetMemory(memory *iface.Memory) {
	m := cloneMemory(memory)
	u.update(func() { u.memory = m })
}

// Usage — единственное место чтения llm.Usage(); вызывается из главной горутины.
func (u *UI) Usage() {
	usage := u.llm.Usage()
	text := fmt.Sprintf("%s, $%0.2f", tokens(int64(usage.TotalTokens)), usage.Cost)
	u.update(func() { u.usageText = text })
}

// Close идемпотентен и не ждёт eventLoop дольше closeTimeout.
func (u *UI) Close() {
	u.shutdown(true)
}

func (u *UI) shutdown(waitLoop bool) {
	u.closeOnce.Do(func() {
		u.mu.Lock()
		u.closed = true
		tail := u.plainTail(tailLines)
		u.mu.Unlock()

		close(u.done)
		tui.SetPrintfHandler(nil)
		u.redraw()
		if waitLoop {
			select {
			case <-u.loopDone:
			case <-time.After(closeTimeout):
			}
		}
		u.fini()

		for _, l := range tail {
			_, _ = fmt.Fprintln(os.Stdout, l)
		}
	})
}
