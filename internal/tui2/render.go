package tui2

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

const (
	inputHint = "Enter — отправить, Alt+Enter — новая строка, Tab — панели, Ctrl+C — выход"
	askHint   = "←/→ — выбор, Enter — подтвердить, 1-9 — быстрый выбор"
)

// putCells рисует s с x, пропуская первые skip ячеек и не выходя за maxW.
// Широкий символ, разрезанный границей, заменяется пробелом. Возвращает
// число ячеек после skip (включая вылезшие за maxW).
func (u *UI) putCells(x, y, maxW, skip int, s string, style tcell.Style) int {
	cx, state := 0, -1
	for s != "" {
		var g string
		var gw int
		g, s, gw, state = uniseg.FirstGraphemeClusterInString(s, state)
		if gw == 0 {
			continue
		}
		start, end := cx-skip, cx+gw-skip
		cx += gw
		if end <= 0 {
			continue
		}
		if start >= maxW {
			continue
		}
		if start < 0 || end > maxW {
			for i := max(start, 0); i < min(end, maxW); i++ {
				u.screen.Put(x+i, y, " ", style)
			}
			continue
		}
		u.screen.Put(x+start, y, g, style)
	}
	return max(cx-skip, 0)
}

func (u *UI) put(x, y, maxW int, s string, style tcell.Style) int {
	return min(u.putCells(x, y, maxW, 0, s, style), max(maxW, 0))
}

func (u *UI) titleStyle(p panel) tcell.Style {
	if u.focus == p && u.ask == nil {
		return styleBold
	}
	if p == panelInput && u.ask != nil {
		return styleQuestion.Bold(true)
	}
	return styleDim
}

func (u *UI) spinner() string {
	return string(spinnerFrames[u.frame%len(spinnerFrames)])
}

// draw перерисовывает весь экран; вызывается только из eventLoop под mu.
func (u *UI) draw() {
	w, h := u.screen.Size()
	u.lay = computeLayout(w, h)
	u.expireStatus(time.Now())
	u.screen.Clear()

	l := u.lay
	if l.tooSmall {
		msg := "Окно слишком маленькое"
		mw := strWidth(msg)
		u.put(max((w-mw)/2, 0), h/2, w, msg, styleWhite)
		u.screen.HideCursor()
		u.screen.Show()
		return
	}

	drawFrame(u.screen, l)
	u.drawChat()
	u.drawPlan()
	u.drawMemory()
	u.drawUsage()
	if u.ask != nil {
		u.drawAsk()
	} else {
		u.drawInput()
	}
	u.drawTitles()
	u.screen.Show()
}

func (u *UI) drawTitles() {
	l := u.lay
	mid := l.leftW + 1

	// Над чатом: статус и рабочая директория.
	status, spin := u.statusLine()
	head := "[ "
	if spin {
		head += u.spinner() + " "
	}
	head += status + " │ "
	room := l.leftW - 2 - strWidth(head) - 2
	title := head + truncateLeft(u.workDir, max(room, 1)) + " ]"
	u.put(2, 0, l.leftW-1, title, u.titleStyle(panelChat))

	planTitle := fmt.Sprintf("[ План %d/%d ]", len(u.plan.Completed()), len(u.plan.Items))
	u.put(mid+2, 0, l.rightW-1, planTitle, u.titleStyle(panelPlan))

	if l.memH > 0 {
		memTitle := fmt.Sprintf("[ Память (%d) ]", len(u.memory.Items))
		u.put(mid+2, 1+l.planH, l.rightW-1, memTitle, u.titleStyle(panelMemory))
	}

	sepY := l.h - 5
	inTitle := "[ Ввод ]"
	if u.ask != nil {
		inTitle = "[ Выбор: ←/→, Enter ]"
	}
	used := u.put(2, sepY, l.leftW-1, inTitle, u.titleStyle(panelInput))
	u.put(mid+2, sepY, l.rightW-1, "[ Токены ]", styleDim)

	if !u.chatFollow && u.newWhileScrolled {
		mark := "[ ↓ новые ]"
		mw := strWidth(mark)
		x := mid - 1 - mw
		if x > 2+used {
			u.put(x, sepY, mw, mark, styleNewLines)
		}
	}
}

func (u *UI) entryPrefix(e *chatEntry) []segment {
	switch e.kind {
	case kindToolRunning:
		return []segment{{u.spinner() + " ", styleToolRun}}
	case kindToolOK:
		return []segment{{"✓ ", styleToolOK}}
	case kindToolFail:
		return []segment{{"✗ ", styleRed}}
	}
	return e.prefix
}

// ensureWrapped пересчитывает перенос только при смене ширины.
func ensureWrapped(e *chatEntry, width int) {
	if e.wrappedW == width && e.wrapped != nil {
		return
	}
	indent := e.prefixW
	if indent > width/2 {
		indent = 0
	}
	tw := max(width-indent, 1)
	e.wrapped = wrap(e.text, tw)
	if indent == 0 && e.prefixW > 0 {
		// Широкий префикс — отдельной строкой.
		e.wrapped = append([]string{""}, e.wrapped...)
	}
	e.wrappedDetail = nil
	if e.detail != "" {
		e.wrappedDetail = wrap(e.detail, tw)
	}
	e.wrappedW = width
}

func (u *UI) drawChat() {
	r := u.lay.chat
	total := 0
	for _, e := range u.chat {
		ensureWrapped(e, r.w)
		total += e.lineCount()
	}
	u.lines[panelChat] = total
	m := max(total-r.h, 0)
	if u.chatFollow {
		u.scroll[panelChat] = m
	}
	u.scroll[panelChat] = min(max(u.scroll[panelChat], 0), m)

	skip := u.scroll[panelChat]
	y := 0
	for _, e := range u.chat {
		if y >= r.h {
			break
		}
		n := e.lineCount()
		if skip >= n {
			skip -= n
			continue
		}
		indent := e.prefixW
		if indent > r.w/2 {
			indent = 0
		}
		for i := skip; i < n && y < r.h; i++ {
			x := r.x
			if i < len(e.wrapped) {
				if i == 0 {
					for _, p := range u.entryPrefix(e) {
						x += u.put(x, r.y+y, r.x+r.w-x, p.text, p.style)
					}
				}
				u.put(max(x, r.x+indent), r.y+y, r.x+r.w-max(x, r.x+indent), e.wrapped[i], e.style)
			} else {
				u.put(r.x+indent, r.y+y, r.w-indent, e.wrappedDetail[i-len(e.wrapped)], styleRed)
			}
			y++
		}
		skip = 0
	}
}

type styledLine struct {
	segs []segment
}

func (u *UI) drawLines(p panel, r rect, lines []styledLine) {
	u.lines[p] = len(lines)
	m := max(len(lines)-r.h, 0)
	u.scroll[p] = min(max(u.scroll[p], 0), m)
	for i := 0; i < r.h && u.scroll[p]+i < len(lines); i++ {
		x := r.x
		for _, s := range lines[u.scroll[p]+i].segs {
			x += u.put(x, r.y+i, r.x+r.w-x, s.text, s.style)
		}
	}
}

func (u *UI) drawPlan() {
	r := u.lay.plan
	if len(u.plan.Items) == 0 {
		u.drawLines(panelPlan, r, []styledLine{{[]segment{{"(пусто)", styleGray}}}})
		return
	}
	nw := planWidth(u.plan)
	var lines []styledLine
	for _, item := range u.plan.Items {
		head := fmt.Sprintf("%*d) ", nw, item.Index)
		indent := strWidth(head) + 2
		if indent > r.w/2 {
			indent = 0
		}
		parts := wrap(item.Title, max(r.w-indent, 1))
		for i, part := range parts {
			if i == 0 {
				lines = append(lines, styledLine{[]segment{{head, styleWhite}, planMark(item), {" ", styleWhite}, {part, styleWhite}}})
			} else {
				lines = append(lines, styledLine{[]segment{{strings.Repeat(" ", indent) + part, styleWhite}}})
			}
		}
	}
	u.drawLines(panelPlan, r, lines)
}

func (u *UI) drawMemory() {
	if u.lay.memH == 0 {
		u.lines[panelMemory] = 0
		return
	}
	r := u.lay.memory
	if len(u.memory.Items) == 0 {
		u.drawLines(panelMemory, r, []styledLine{{[]segment{{"(пусто)", styleGray}}}})
		return
	}
	lines := make([]styledLine, 0, len(u.memory.Items))
	for _, item := range u.memory.Items {
		key, _, _ := strings.Cut(item.Key, "\n")
		lines = append(lines, styledLine{[]segment{{truncate("• "+key, r.w), styleWhite}}})
	}
	u.drawLines(panelMemory, r, lines)
}

func (u *UI) drawUsage() {
	r := u.lay.usage
	text := u.usageText
	if text == "" {
		text = "—"
	}
	text = truncate(text, r.w)
	u.put(r.x+r.w-strWidth(text), r.y, r.w, text, styleDim)
}

func (u *UI) drawInput() {
	r := u.lay.input
	in := &u.input
	if in.viewW != r.w {
		// Только при смене ширины: иначе сбросили бы скролл колесом.
		in.ensureVisible(r.w)
	}

	if in.isEmpty() {
		u.put(r.x, r.y, r.w, truncate(inputHint, r.w), styleGray)
	} else {
		for i := 0; i < inputLines && in.top+i < len(in.lines); i++ {
			line := string(in.lines[in.top+i])
			cells := u.putCells(r.x, r.y+i, r.w, in.left, line, styleWhite)
			if in.left > 0 {
				u.screen.Put(r.x, r.y+i, "‹", styleDim)
			}
			if cells > r.w {
				u.screen.Put(r.x+r.w-1, r.y+i, "›", styleDim)
			}
		}
	}

	cx, cy := in.cursorX()-in.left, in.row-in.top
	if u.focus == panelInput && cx >= 0 && cx < r.w && cy >= 0 && cy < inputLines {
		u.screen.ShowCursor(r.x+cx, r.y+cy)
	} else {
		u.screen.HideCursor()
	}
}

func (u *UI) drawAsk() {
	r := u.lay.input
	a := u.ask
	u.screen.HideCursor()

	q, _, _ := strings.Cut(a.question, "\n")
	u.put(r.x, r.y, r.w, truncate(q, r.w), styleQuestion)

	type opt struct {
		text  string
		style tcell.Style
	}
	opts := make([]opt, 0, len(a.options))
	selStart, selEnd, pos := 0, 0, 0
	for i, o := range a.options {
		o, _, _ = strings.Cut(o, "\n")
		item := opt{" " + o + " ", styleQuestion}
		if i == a.selected {
			item = opt{"<" + o + ">", styleQuestion.Underline(true).Reverse(true)}
			selStart = pos
			selEnd = pos + strWidth(item.text)
		}
		opts = append(opts, item)
		pos += strWidth(item.text) + 1
	}
	skip := 0
	if selEnd > r.w {
		skip = selEnd - r.w
	}
	skip = min(skip, selStart)

	x := 0
	for _, o := range opts {
		ow := strWidth(o.text)
		u.putCells(r.x, r.y+1, r.w, skip-x, o.text, o.style)
		x += ow + 1
	}

	u.put(r.x, r.y+2, r.w, truncate(askHint, r.w), styleGray)
}
