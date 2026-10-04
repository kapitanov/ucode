package tui2

import (
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// inputState — многострочный редактор поля ввода.
type inputState struct {
	lines       [][]rune
	row, col    int
	top, left   int
	pasting     bool
	pastedCR    bool
	pastedRunes int // сколько рун уже вставлено за текущую пасту
	viewW       int // ширина окна, под которую подогнан скролл
}

func newInputState() inputState {
	return inputState{lines: [][]rune{{}}}
}

func (in *inputState) isEmpty() bool {
	return len(in.lines) == 1 && len(in.lines[0]) == 0
}

func (in *inputState) text() string {
	parts := make([]string, len(in.lines))
	for i, l := range in.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (in *inputState) clear() {
	*in = inputState{lines: [][]rune{{}}, pasting: in.pasting}
}

func (in *inputState) insert(r rune) {
	line := in.lines[in.row]
	line = append(line[:in.col], append([]rune{r}, line[in.col:]...)...)
	in.lines[in.row] = line
	in.col++
}

func (in *inputState) newline() {
	line := in.lines[in.row]
	tail := append([]rune(nil), line[in.col:]...)
	in.lines[in.row] = line[:in.col]
	in.lines = append(in.lines[:in.row+1], append([][]rune{tail}, in.lines[in.row+1:]...)...)
	in.row++
	in.col = 0
}

func (in *inputState) backspace() {
	if in.col > 0 {
		line := in.lines[in.row]
		in.lines[in.row] = append(line[:in.col-1], line[in.col:]...)
		in.col--
		return
	}
	if in.row > 0 {
		prev := in.lines[in.row-1]
		in.col = len(prev)
		in.lines[in.row-1] = append(prev, in.lines[in.row]...)
		in.lines = append(in.lines[:in.row], in.lines[in.row+1:]...)
		in.row--
	}
}

func (in *inputState) del() {
	line := in.lines[in.row]
	if in.col < len(line) {
		in.lines[in.row] = append(line[:in.col], line[in.col+1:]...)
		return
	}
	if in.row < len(in.lines)-1 {
		in.lines[in.row] = append(line, in.lines[in.row+1]...)
		in.lines = append(in.lines[:in.row+1], in.lines[in.row+2:]...)
	}
}

func (in *inputState) moveLeft() {
	if in.col > 0 {
		in.col--
	} else if in.row > 0 {
		in.row--
		in.col = len(in.lines[in.row])
	}
}

func (in *inputState) moveRight() {
	if in.col < len(in.lines[in.row]) {
		in.col++
	} else if in.row < len(in.lines)-1 {
		in.row++
		in.col = 0
	}
}

func (in *inputState) moveRow(d int) {
	in.row = min(max(in.row+d, 0), len(in.lines)-1)
	in.col = min(in.col, len(in.lines[in.row]))
}

// cursorX — позиция курсора в ячейках от начала строки.
func (in *inputState) cursorX() int {
	return strWidth(string(in.lines[in.row][:in.col]))
}

// ensureVisible подгоняет скролл под курсор.
func (in *inputState) ensureVisible(width int) {
	width = max(width, 1)
	in.viewW = width
	if in.row < in.top {
		in.top = in.row
	}
	if in.row >= in.top+inputLines {
		in.top = in.row - inputLines + 1
	}
	cx := in.cursorX()
	if cx < in.left {
		in.left = cx
	}
	if cx >= in.left+width {
		in.left = cx - width + 1
	}
}

func (in *inputState) maxLineWidth() int {
	w := 0
	for _, l := range in.lines {
		w = max(w, strWidth(string(l)))
	}
	return w
}

func allowedRune(r rune) bool {
	return !unicode.IsControl(r) && (r < 0x202A || r > 0x202E) && (r < 0x2066 || r > 0x2069)
}

const ctrlCWindow = 2 * time.Second

// handleEvent обрабатывает событие под mu; true — аварийный выход.
func (u *UI) handleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		u.screen.Sync()
	case *tcell.EventPaste:
		u.input.pasting = ev.Start()
		u.input.pastedCR = false
		u.input.pastedRunes = 0
	case *tcell.EventMouse:
		u.handleMouse(ev)
	case *tcell.EventKey:
		if u.input.pasting {
			u.handlePasteKey(ev)
			return false
		}
		return u.handleKey(ev)
	}
	return false
}

// maxPasteRunes — максимальная длина вставки (1024 руна)
const maxPasteRunes = 1024

func (u *UI) handlePasteKey(ev *tcell.EventKey) {
	in := &u.input
	cr := false
	switch ev.Key() {
	case tcell.KeyRune:
		// Ограничиваем длину вставки 1024 рунами
		if in.pastedRunes >= maxPasteRunes {
			return
		}
		if allowedRune(ev.Rune()) {
			in.insert(ev.Rune())
			in.pastedRunes++
		}
	case tcell.KeyEnter:
		// Терминалы обычно шлют перевод строки при вставке как '\r'.
		in.newline()
		cr = true
	case tcell.KeyCtrlJ:
		if !in.pastedCR {
			in.newline()
		}
	case tcell.KeyTab:
		for i := 0; i < 4 && in.pastedRunes < maxPasteRunes; i++ {
			in.insert(' ')
			in.pastedRunes++
		}
	}
	in.pastedCR = cr
	in.ensureVisible(u.lay.input.w)
}

func (u *UI) handleKey(ev *tcell.EventKey) bool {
	key, mod := ev.Key(), ev.Modifiers()

	switch key {
	case tcell.KeyCtrlC:
		return u.ctrlC()
	case tcell.KeyEsc:
		u.focus = panelInput
		return false
	}

	if u.ask != nil {
		u.handleAskKey(ev)
		return false
	}

	switch key {
	case tcell.KeyTab:
		u.focus = (u.focus + 1) % panelCount
		return false
	case tcell.KeyBacktab:
		u.focus = (u.focus + panelCount - 1) % panelCount
		return false
	}

	if u.focus != panelInput {
		u.handlePanelKey(u.focus, key)
		return false
	}

	in := &u.input
	switch key {
	case tcell.KeyRune:
		if allowedRune(ev.Rune()) {
			in.insert(ev.Rune())
		}
	case tcell.KeyEnter:
		if mod&tcell.ModAlt != 0 {
			in.newline()
		} else {
			u.submit()
		}
	case tcell.KeyCtrlJ:
		in.newline()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		in.backspace()
	case tcell.KeyDelete:
		in.del()
	case tcell.KeyCtrlD:
		if in.isEmpty() {
			return u.ctrlC()
		}
		in.del()
	case tcell.KeyLeft:
		in.moveLeft()
	case tcell.KeyRight:
		in.moveRight()
	case tcell.KeyUp:
		in.moveRow(-1)
	case tcell.KeyDown:
		in.moveRow(1)
	case tcell.KeyHome:
		if mod&tcell.ModCtrl != 0 {
			u.scrollHome(panelChat)
		} else {
			in.col = 0
		}
	case tcell.KeyEnd:
		if mod&tcell.ModCtrl != 0 {
			u.scrollEnd(panelChat)
		} else {
			in.col = len(in.lines[in.row])
		}
	case tcell.KeyCtrlA:
		in.col = 0
	case tcell.KeyCtrlE:
		in.col = len(in.lines[in.row])
	case tcell.KeyCtrlU:
		in.clear()
	case tcell.KeyPgUp:
		u.scrollBy(panelChat, -u.pageSize(panelChat))
	case tcell.KeyPgDn:
		u.scrollBy(panelChat, u.pageSize(panelChat))
	}
	in.ensureVisible(u.lay.input.w)
	return false
}

func (u *UI) handlePanelKey(p panel, key tcell.Key) {
	switch key {
	case tcell.KeyUp:
		u.scrollBy(p, -1)
	case tcell.KeyDown:
		u.scrollBy(p, 1)
	case tcell.KeyPgUp:
		u.scrollBy(p, -u.pageSize(p))
	case tcell.KeyPgDn:
		u.scrollBy(p, u.pageSize(p))
	case tcell.KeyHome:
		u.scrollHome(p)
	case tcell.KeyEnd:
		u.scrollEnd(p)
	}
}

func (u *UI) handleAskKey(ev *tcell.EventKey) {
	a := u.ask
	switch ev.Key() {
	case tcell.KeyLeft, tcell.KeyBacktab:
		a.selected = max(a.selected-1, 0)
	case tcell.KeyRight, tcell.KeyTab:
		a.selected = min(a.selected+1, len(a.options)-1)
	case tcell.KeyEnter:
		u.answer(a.selected)
	case tcell.KeyRune:
		if r := ev.Rune(); r >= '1' && r <= '9' && int(r-'1') < len(a.options) {
			a.selected = int(r - '1')
			u.answer(a.selected)
		}
	case tcell.KeyPgUp:
		u.scrollBy(panelChat, -u.pageSize(panelChat))
	case tcell.KeyPgDn:
		u.scrollBy(panelChat, u.pageSize(panelChat))
	}
}

// answer отправляет ответ Ask; канал буферизован, повтор отсекается флагом.
func (u *UI) answer(i int) {
	if u.ask == nil || u.ask.answered {
		return
	}
	u.ask.answered = true
	u.ask.ch <- i
}

func (u *UI) submit() {
	text := sanitize(strings.TrimSpace(u.input.text()))
	if text == "" {
		return
	}
	if u.promptCh == nil {
		_ = u.screen.Beep()
		return
	}
	u.input.clear()
	if strings.HasPrefix(text, "/") {
		if u.runCommand(text) {
			u.sendPrompt("")
		}
		return
	}
	u.addEntry(newEntry([]segment{{"> ", styleGreen.Bold(true)}}, text, styleWhite))
	u.chatFollow = true
	u.newWhileScrolled = false
	u.sendPrompt(text)
}

// sendPrompt — канал буферизован (1), после отправки обнуляется.
// Вызывается из submit()/ctrlC() внутри lock-а step() — u.promptCh безопасен.
// Вызывается из submit()/ctrlC() внутри lock-а step() — u.promptCh безопасен.
func (u *UI) sendPrompt(s string) {
	if u.promptCh == nil {
		return
	}
	u.promptCh <- s
	u.promptCh = nil
}

// ctrlC — запрос выхода; true — повторный Ctrl+C во время работы агента.
// Вызывается из handleKey() внутри lock-а step() — u.promptCh и u.ask безопасны.
// Вызывается из handleKey() внутри lock-а step() — u.promptCh и u.ask безопасны.
func (u *UI) ctrlC() bool {
	now := time.Now()
	defer func() { u.lastCtrlC = now }()

	if u.ask != nil && !u.ask.answered {
		u.quitRequested = true
		u.answer(safeChoice(u.ask.options))
		return false
	}
	if u.promptCh != nil {
		u.quitRequested = true
		u.sendPrompt("")
		return false
	}
	if u.quitRequested && now.Sub(u.lastCtrlC) < ctrlCWindow {
		return true
	}
	u.quitRequested = true
	return false
}

func (u *UI) handleMouse(ev *tcell.EventMouse) {
	x, y := ev.Position()
	p, ok := u.lay.panelAt(x, y)
	if !ok {
		return
	}
	b := ev.Buttons()
	switch {
	case b&tcell.Button1 != 0:
		u.focus = p
	case b&tcell.WheelUp != 0:
		if p != panelInput {
			u.scrollBy(p, -3)
		}
	case b&tcell.WheelDown != 0:
		if p != panelInput {
			u.scrollBy(p, 3)
		}
	case b&tcell.WheelLeft != 0:
		if p == panelInput {
			u.input.left = max(u.input.left-3, 0)
		}
	case b&tcell.WheelRight != 0:
		if p == panelInput {
			u.input.left = min(u.input.left+3, max(u.input.maxLineWidth()-1, 0))
		}
	}
}

func (u *UI) pageSize(p panel) int {
	return max(u.lay.panelRect(p).h-1, 1)
}

func (u *UI) maxScroll(p panel) int {
	return max(u.lines[p]-u.lay.panelRect(p).h, 0)
}

func (u *UI) scrollBy(p panel, d int) {
	m := u.maxScroll(p)
	if p == panelChat && u.chatFollow {
		u.scroll[p] = m
	}
	u.scroll[p] = min(max(u.scroll[p]+d, 0), m)
	if p == panelChat {
		u.updateFollow(m)
	}
}

func (u *UI) scrollHome(p panel) {
	u.scroll[p] = 0
	if p == panelChat {
		u.updateFollow(u.maxScroll(p))
	}
}

func (u *UI) scrollEnd(p panel) {
	u.scroll[p] = u.maxScroll(p)
	if p == panelChat {
		u.updateFollow(u.scroll[p])
	}
}

func (u *UI) updateFollow(maxScroll int) {
	if u.scroll[panelChat] >= maxScroll {
		u.chatFollow = true
		u.newWhileScrolled = false
	} else {
		u.chatFollow = false
	}
}
