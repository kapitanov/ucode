package tui2

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// newEditorUI — UI без экрана и горутин: только редактор и раскладка 80x24.
func newEditorUI() *UI {
	return &UI{
		input:      newInputState(),
		lay:        computeLayout(80, 24),
		focus:      panelInput,
		chatFollow: true,
		baseStatus: statusIdle,
	}
}

func key(u *UI, k tcell.Key, mod tcell.ModMask) {
	u.handleEvent(tcell.NewEventKey(k, 0, mod))
}

func typeText(u *UI, s string) {
	for _, r := range s {
		u.handleEvent(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func lines(u *UI) []string {
	out := make([]string, len(u.input.lines))
	for i, l := range u.input.lines {
		out[i] = string(l)
	}
	return out
}

func checkInput(t *testing.T, u *UI, want []string, row, col int) {
	t.Helper()
	if got := lines(u); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	if u.input.row != row || u.input.col != col {
		t.Fatalf("cursor = (%d,%d), want (%d,%d)", u.input.row, u.input.col, row, col)
	}
}

func TestEditorInsertAndMove(t *testing.T) {
	u := newEditorUI()
	typeText(u, "привет")
	checkInput(t, u, []string{"привет"}, 0, 6)

	key(u, tcell.KeyLeft, 0)
	key(u, tcell.KeyLeft, 0)
	typeText(u, "X")
	checkInput(t, u, []string{"привXет"}, 0, 5)

	key(u, tcell.KeyHome, 0)
	typeText(u, "<")
	checkInput(t, u, []string{"<привXет"}, 0, 1)
	key(u, tcell.KeyEnd, 0)
	typeText(u, ">")
	checkInput(t, u, []string{"<привXет>"}, 0, 9)
	key(u, tcell.KeyCtrlA, tcell.ModCtrl)
	checkInput(t, u, []string{"<привXет>"}, 0, 0)
	key(u, tcell.KeyCtrlE, tcell.ModCtrl)
	checkInput(t, u, []string{"<привXет>"}, 0, 9)

	// Влево в начале и вправо в конце единственной строки — без паники.
	key(u, tcell.KeyRight, 0)
	checkInput(t, u, []string{"<привXет>"}, 0, 9)
	key(u, tcell.KeyHome, 0)
	key(u, tcell.KeyLeft, 0)
	checkInput(t, u, []string{"<привXет>"}, 0, 0)

	// Управляющие и bidi-руны не вставляются.
	typeText(u, "\u202e\u0007")
	checkInput(t, u, []string{"<привXет>"}, 0, 0)

	key(u, tcell.KeyCtrlU, tcell.ModCtrl)
	checkInput(t, u, []string{""}, 0, 0)
	if !u.input.isEmpty() {
		t.Fatal("must be empty after Ctrl+U")
	}
}

func TestEditorNewlines(t *testing.T) {
	u := newEditorUI()
	typeText(u, "abcd")
	key(u, tcell.KeyLeft, 0)
	key(u, tcell.KeyLeft, 0)
	key(u, tcell.KeyEnter, tcell.ModAlt)
	checkInput(t, u, []string{"ab", "cd"}, 1, 0)
	key(u, tcell.KeyEnd, 0)
	key(u, tcell.KeyCtrlJ, tcell.ModCtrl)
	typeText(u, "ef")
	checkInput(t, u, []string{"ab", "cd", "ef"}, 2, 2)

	// Left/Right переходят между строками.
	key(u, tcell.KeyHome, 0)
	key(u, tcell.KeyLeft, 0)
	checkInput(t, u, []string{"ab", "cd", "ef"}, 1, 2)
	key(u, tcell.KeyRight, 0)
	checkInput(t, u, []string{"ab", "cd", "ef"}, 2, 0)

	// Up/Down с клампом колонки.
	key(u, tcell.KeyUp, 0)
	key(u, tcell.KeyUp, 0)
	key(u, tcell.KeyUp, 0)
	checkInput(t, u, []string{"ab", "cd", "ef"}, 0, 0)
	key(u, tcell.KeyEnd, 0)
	typeText(u, "zzz")
	key(u, tcell.KeyDown, 0)
	checkInput(t, u, []string{"abzzz", "cd", "ef"}, 1, 2)
	key(u, tcell.KeyDown, 0)
	key(u, tcell.KeyDown, 0)
	checkInput(t, u, []string{"abzzz", "cd", "ef"}, 2, 2)
	if got := u.input.text(); got != "abzzz\ncd\nef" {
		t.Fatalf("text = %q", got)
	}
}

func TestEditorBackspaceDelete(t *testing.T) {
	u := newEditorUI()
	typeText(u, "ab")
	key(u, tcell.KeyEnter, tcell.ModAlt)
	typeText(u, "cd")
	key(u, tcell.KeyHome, 0)

	// Backspace в начале строки склеивает с предыдущей.
	key(u, tcell.KeyBackspace2, 0)
	checkInput(t, u, []string{"abcd"}, 0, 2)
	key(u, tcell.KeyBackspace, 0)
	checkInput(t, u, []string{"acd"}, 0, 1)

	// Delete в конце строки склеивает со следующей.
	key(u, tcell.KeyEnd, 0)
	key(u, tcell.KeyEnter, tcell.ModAlt)
	typeText(u, "ef")
	key(u, tcell.KeyUp, 0)
	key(u, tcell.KeyEnd, 0)
	key(u, tcell.KeyDelete, 0)
	checkInput(t, u, []string{"acdef"}, 0, 3)
	key(u, tcell.KeyDelete, 0)
	checkInput(t, u, []string{"acdf"}, 0, 3)

	// Ctrl+D на непустом поле — Delete.
	key(u, tcell.KeyHome, 0)
	key(u, tcell.KeyCtrlD, tcell.ModCtrl)
	checkInput(t, u, []string{"cdf"}, 0, 0)
	if u.quitRequested {
		t.Fatal("Ctrl+D on non-empty input must not quit")
	}

	// Края: Backspace в самом начале и Delete в самом конце — no-op.
	key(u, tcell.KeyBackspace2, 0)
	checkInput(t, u, []string{"cdf"}, 0, 0)
	key(u, tcell.KeyEnd, 0)
	key(u, tcell.KeyDelete, 0)
	checkInput(t, u, []string{"cdf"}, 0, 3)

	// Склейка после newline в середине строки не портит хвост (алиасинг слайсов).
	key(u, tcell.KeyHome, 0)
	key(u, tcell.KeyRight, 0)
	key(u, tcell.KeyEnter, tcell.ModAlt)
	checkInput(t, u, []string{"c", "df"}, 1, 0)
	key(u, tcell.KeyUp, 0)
	key(u, tcell.KeyEnd, 0)
	typeText(u, "XYZ")
	checkInput(t, u, []string{"cXYZ", "df"}, 0, 4)
	key(u, tcell.KeyDelete, 0)
	checkInput(t, u, []string{"cXYZdf"}, 0, 4)
}

func TestEditorHorizontalScroll(t *testing.T) {
	u := newEditorUI()
	w := u.lay.input.w
	typeText(u, strings.Repeat("a", w+10))
	if want := w + 10 - w + 1; u.input.left != want {
		t.Fatalf("left = %d, want %d", u.input.left, want)
	}
	cx := u.input.cursorX() - u.input.left
	if cx < 0 || cx >= w {
		t.Fatalf("cursor at %d out of window %d", cx, w)
	}
	key(u, tcell.KeyHome, 0)
	if u.input.left != 0 {
		t.Fatalf("left after Home = %d", u.input.left)
	}
	key(u, tcell.KeyEnd, 0)
	if u.input.left != 11 {
		t.Fatalf("left after End = %d", u.input.left)
	}

	// Широкие символы: курсор считается в ячейках.
	u = newEditorUI()
	typeText(u, strings.Repeat("你", w))
	if u.input.cursorX() != 2*w {
		t.Fatalf("cursorX = %d, want %d", u.input.cursorX(), 2*w)
	}
	if cx := u.input.cursorX() - u.input.left; cx < 0 || cx >= w {
		t.Fatalf("cursor %d out of window", cx)
	}
}

func TestEditorVerticalScroll(t *testing.T) {
	u := newEditorUI()
	for i := 0; i < 6; i++ {
		typeText(u, string(rune('a'+i)))
		if i < 5 {
			key(u, tcell.KeyEnter, tcell.ModAlt)
		}
	}
	if u.input.row != 5 || u.input.top != 3 {
		t.Fatalf("row=%d top=%d, want 5/3", u.input.row, u.input.top)
	}
	key(u, tcell.KeyUp, 0)
	key(u, tcell.KeyUp, 0)
	if u.input.top != 3 {
		t.Fatalf("top=%d, want 3", u.input.top)
	}
	key(u, tcell.KeyUp, 0)
	if u.input.row != 2 || u.input.top != 2 {
		t.Fatalf("row=%d top=%d, want 2/2", u.input.row, u.input.top)
	}
	for i := 0; i < 5; i++ {
		key(u, tcell.KeyUp, 0)
	}
	if u.input.row != 0 || u.input.top != 0 {
		t.Fatalf("row=%d top=%d, want 0/0", u.input.row, u.input.top)
	}
}

func paste(u *UI, evs ...*tcell.EventKey) {
	u.handleEvent(tcell.NewEventPaste(true))
	for _, ev := range evs {
		u.handleEvent(ev)
	}
	u.handleEvent(tcell.NewEventPaste(false))
}

func runeEvents(s string) []*tcell.EventKey {
	var evs []*tcell.EventKey
	for _, r := range s {
		// Так же, как inputProcessor tcell превращает байты терминала в события.
		switch {
		case r == '\t':
			evs = append(evs, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		case r == '\r':
			evs = append(evs, tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		case r < ' ':
			evs = append(evs, tcell.NewEventKey(tcell.KeyCtrlSpace+tcell.Key(r), 0, tcell.ModCtrl))
		default:
			evs = append(evs, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
		}
	}
	return evs
}

func TestEditorPaste(t *testing.T) {
	u := newEditorUI()
	ch := make(chan string, 1)
	u.promptCh = ch // Enter в режиме вставки не должен отправлять.

	typeText(u, ">")
	paste(u, runeEvents("line1\r\nline2\nline3\rline4\tend\x07\x15\x03")...)
	checkInput(t, u, []string{">line1", "line2", "line3", "line4    end"}, 3, 12)
	if u.input.pasting {
		t.Fatal("pasting must be reset after paste end")
	}
	select {
	case s := <-ch:
		t.Fatalf("paste submitted %q", s)
	default:
	}
	if u.promptCh == nil {
		t.Fatal("prompt must still wait")
	}
	if u.input.top != 1 {
		t.Fatalf("top = %d, want 1 (cursor visible)", u.input.top)
	}

	// После вставки обычный Enter снова отправляет.
	key(u, tcell.KeyEnter, 0)
	select {
	case s := <-ch:
		if s != ">line1\nline2\nline3\nline4    end" {
			t.Fatalf("submitted %q", s)
		}
	default:
		t.Fatal("Enter after paste must submit")
	}
}

func TestEditorPasteCRLFPairs(t *testing.T) {
	u := newEditorUI()
	paste(u,
		tcell.NewEventKey(tcell.KeyRune, 'a', 0),
		tcell.NewEventKey(tcell.KeyEnter, 0, 0),
		tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyRune, 'b', 0),
		tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyRune, 'c', 0),
		tcell.NewEventKey(tcell.KeyTab, 0, 0),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, 0),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl),
	)
	checkInput(t, u, []string{"a", "b", "", "c    "}, 3, 5)
	if u.quitRequested {
		t.Fatal("Ctrl+C inside paste must be ignored")
	}
}

// Случайные нажатия: курсор и скролл всегда валидны, паник нет.
func TestEditorRandom(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	keys := []tcell.Key{tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown, tcell.KeyHome, tcell.KeyEnd,
		tcell.KeyBackspace2, tcell.KeyDelete, tcell.KeyCtrlJ, tcell.KeyCtrlA, tcell.KeyCtrlE}
	runes := []rune("ab я你👍 ")
	for iter := 0; iter < 20; iter++ {
		u := newEditorUI()
		if iter%2 == 1 {
			u.lay = computeLayout(40, 10)
		}
		w := u.lay.input.w
		for i := 0; i < 2000; i++ {
			switch rnd.Intn(4) {
			case 0, 1:
				typeText(u, string(runes[rnd.Intn(len(runes))]))
			case 2:
				key(u, keys[rnd.Intn(len(keys))], 0)
			case 3:
				if rnd.Intn(50) == 0 {
					key(u, tcell.KeyCtrlU, tcell.ModCtrl)
				} else {
					key(u, keys[rnd.Intn(len(keys))], 0)
				}
			}
			in := &u.input
			if in.row < 0 || in.row >= len(in.lines) || in.col < 0 || in.col > len(in.lines[in.row]) {
				t.Fatalf("cursor (%d,%d) out of buffer %q", in.row, in.col, lines(u))
			}
			if in.row < in.top || in.row >= in.top+inputLines {
				t.Fatalf("row %d not visible (top %d)", in.row, in.top)
			}
			if cx := in.cursorX() - in.left; cx < 0 || cx >= w {
				t.Fatalf("cursor x %d not visible (left %d, w %d)", cx, in.left, w)
			}
		}
	}
}

func TestPanelFocusAndScroll(t *testing.T) {
	u := newEditorUI()
	key(u, tcell.KeyTab, 0)
	if u.focus != panelChat {
		t.Fatalf("focus = %d", u.focus)
	}
	key(u, tcell.KeyTab, 0)
	key(u, tcell.KeyTab, 0)
	if u.focus != panelMemory {
		t.Fatalf("focus = %d", u.focus)
	}
	key(u, tcell.KeyTab, 0)
	if u.focus != panelInput {
		t.Fatalf("focus = %d", u.focus)
	}
	key(u, tcell.KeyBacktab, 0)
	if u.focus != panelMemory {
		t.Fatalf("focus = %d", u.focus)
	}
	key(u, tcell.KeyEsc, 0)
	if u.focus != panelInput {
		t.Fatalf("focus = %d", u.focus)
	}

	// Прокрутка чата: 100 строк, видно chat.h.
	u.lines[panelChat] = 100
	maxS := 100 - u.lay.chat.h
	key(u, tcell.KeyPgUp, 0)
	if u.chatFollow || u.scroll[panelChat] != maxS-u.pageSize(panelChat) {
		t.Fatalf("PgUp: follow=%v scroll=%d", u.chatFollow, u.scroll[panelChat])
	}
	key(u, tcell.KeyHome, tcell.ModCtrl)
	if u.scroll[panelChat] != 0 || u.chatFollow {
		t.Fatalf("Ctrl+Home: scroll=%d", u.scroll[panelChat])
	}
	u.newWhileScrolled = true
	key(u, tcell.KeyEnd, tcell.ModCtrl)
	if u.scroll[panelChat] != maxS || !u.chatFollow || u.newWhileScrolled {
		t.Fatalf("Ctrl+End: scroll=%d follow=%v", u.scroll[panelChat], u.chatFollow)
	}

	// Колесо над планом не меняет фокус.
	u.lines[panelPlan] = 50
	u.handleEvent(tcell.NewEventMouse(u.lay.plan.x, u.lay.plan.y, tcell.WheelDown, 0))
	if u.scroll[panelPlan] != 3 || u.focus != panelInput {
		t.Fatalf("wheel: scroll=%d focus=%d", u.scroll[panelPlan], u.focus)
	}
	u.handleEvent(tcell.NewEventMouse(u.lay.memory.x, u.lay.memory.y, tcell.Button1, 0))
	if u.focus != panelMemory {
		t.Fatalf("click focus = %d", u.focus)
	}
	// Клик по рамке ничего не делает.
	u.handleEvent(tcell.NewEventMouse(0, 0, tcell.Button1, 0))
	if u.focus != panelMemory {
		t.Fatalf("click on border changed focus to %d", u.focus)
	}
}

