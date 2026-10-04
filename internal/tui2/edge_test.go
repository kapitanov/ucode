package tui2

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kapitanov/ucode/internal/iface"
)

func TestSanitizeNestedEscapes(t *testing.T) {
	cases := map[string]string{
		// ESC внутри CSI начинает новую последовательность, как в терминале.
		"a\x1b[31\x1b[0mb": "ab",
		// ESC ESC: второй ESC начинает новую последовательность.
		"a\x1b\x1b[31mb": "ab",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInputWheelHorizontalScroll(t *testing.T) {
	h := newHarness(t)
	ch := h.prompt()
	long := "START" + strings.Repeat("-", 100) + "END"
	h.typeText(long)
	h.key(tcell.KeyHome, 0)
	h.waitScreen("START")

	var in rect
	h.state(func(u *UI) { in = u.lay.input })
	for i := 0; i < 2; i++ {
		h.screen.InjectMouse(in.x+1, in.y, tcell.WheelRight, 0)
	}
	h.waitFor("input scrolled right", func() bool {
		h.ui.redraw()
		return !strings.Contains(h.screenText(), "START")
	})
	h.state(func(u *UI) {
		if u.input.left != 6 {
			t.Errorf("left = %d, want 6", u.input.left)
		}
	})
	// Курсор, оставшийся левее окна, не рисуется поверх рамки.
	h.waitFor("cursor hidden", func() bool {
		h.ui.redraw()
		x, y, vis := h.screen.GetCursor()
		return !vis || in.contains(x, y)
	})
	// Движение курсора возвращает его в окно.
	h.key(tcell.KeyHome, 0)
	h.waitScreen("START")
	h.key(tcell.KeyEnter, 0)
	if got := recvString(t, ch); got != long {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestDoubleCtrlCExits(t *testing.T) {
	h := newHarness(t)
	codes := make(chan int, 1)
	h.state(func(u *UI) { u.exit = func(code int) { codes <- code } })
	h.key(tcell.KeyCtrlC, tcell.ModCtrl)
	h.waitScreen("Выход после завершения запроса")
	h.key(tcell.KeyCtrlC, tcell.ModCtrl)
	select {
	case code := <-codes:
		if code != 130 {
			t.Fatalf("exit code = %d", code)
		}
	case <-time.After(waitTimeout):
		t.Fatal("exit not called")
	}
	h.state(func(u *UI) {
		if !u.closed {
			t.Error("UI must be closed before exit")
		}
	})
	closeWithTimeout(t, h.ui)
}

func TestAskLongOptionsVisible(t *testing.T) {
	h := newHarness(t)
	opts := make([]string, 8)
	for i := range opts {
		opts[i] = "вариант-номер-" + strconv.Itoa(i) + strings.Repeat("x", 10)
	}
	ch := h.ask("Длинный выбор?\nвторая строка вопроса", opts)
	for i := 0; i < len(opts)-1; i++ {
		h.key(tcell.KeyRight, 0)
	}
	h.waitScreen("<" + opts[7] + ">")
	h.key(tcell.KeyLeft, 0)
	h.waitScreen("<" + opts[6] + ">")
	h.key(tcell.KeyEnter, 0)
	if got := recvInt(t, ch); got != 6 {
		t.Fatalf("Ask = %d", got)
	}
}

func TestChatFollowAndNewMarker(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 60; i++ {
		h.ui.Response("agent", "строка "+strconv.Itoa(i))
	}
	h.waitScreen("строка 59")
	h.key(tcell.KeyTab, 0) // фокус на чат
	h.key(tcell.KeyHome, 0)
	h.waitScreen("agent: строка 0 ")
	h.ui.Response("agent", "новая запись")
	h.waitScreen("↓ новые")
	if strings.Contains(h.screenText(), "новая запись") {
		t.Fatal("chat must not follow while scrolled up")
	}
	h.key(tcell.KeyEnd, 0)
	h.waitScreen("новая запись")
	h.waitFor("marker removed", func() bool {
		h.ui.redraw()
		return !strings.Contains(h.screenText(), "↓ новые")
	})
	h.ui.Response("agent", "ещё одна")
	h.waitScreen("ещё одна")
}

func TestPlanPanelScroll(t *testing.T) {
	h := newHarness(t)
	var items []iface.PlanItem
	for i := 1; i <= 40; i++ {
		items = append(items, iface.PlanItem{Index: i, Title: "шаг " + strconv.Itoa(i)})
	}
	h.ui.SetPlan(&iface.Plan{Items: items})
	h.waitScreen(" 1) ◻ шаг 1 ")
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnd, 0)
	h.waitScreen("40) ◻ шаг 40")
	h.key(tcell.KeyHome, 0)
	h.waitScreen(" 1) ◻ шаг 1 ")
}

