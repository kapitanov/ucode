package tui2

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

// rect — внутренняя область панели без рамок.
type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type layout struct {
	w, h          int
	tooSmall      bool
	leftW, rightW int
	planH, memH   int
	chat, plan    rect
	memory, input rect
	usage         rect
}

const (
	minWidth   = 40
	minHeight  = 10
	inputLines = 3
)

func computeLayout(w, h int) layout {
	l := layout{w: w, h: h}
	if w < minWidth || h < minHeight {
		l.tooSmall = true
		return l
	}

	l.rightW = min(max(w*30/100, 24), 50)
	l.leftW = w - l.rightW - 3
	x0 := l.leftW + 2
	topH := h - 6

	l.planH = max(1, (topH-1)*55/100)
	l.memH = topH - 1 - l.planH

	l.chat = rect{1, 1, l.leftW, topH}
	l.plan = rect{x0, 1, l.rightW, l.planH}
	if l.memH >= 1 {
		l.memory = rect{x0, 2 + l.planH, l.rightW, l.memH}
	} else {
		l.memH = 0
	}
	l.input = rect{1, h - 4, l.leftW, inputLines}
	l.usage = rect{x0, h - 3, l.rightW, 1}
	return l
}

// panelAt — панель под точкой (для мыши).
func (l layout) panelAt(x, y int) (panel, bool) {
	if l.tooSmall {
		return 0, false
	}
	switch {
	case l.chat.contains(x, y):
		return panelChat, true
	case l.plan.contains(x, y):
		return panelPlan, true
	case l.memH > 0 && l.memory.contains(x, y):
		return panelMemory, true
	case l.input.contains(x, y):
		return panelInput, true
	}
	return 0, false
}

func (l layout) panelRect(p panel) rect {
	switch p {
	case panelChat:
		return l.chat
	case panelPlan:
		return l.plan
	case panelMemory:
		return l.memory
	}
	return l.input
}

// drawFrame рисует общие рамки панелей.
func drawFrame(s tcell.Screen, l layout) {
	w, h := l.w, l.h
	mid := l.leftW + 1
	sepY := h - 5
	planSepY := 1 + l.planH

	hline := func(y, x1, x2 int) {
		for x := x1; x <= x2; x++ {
			s.SetContent(x, y, tcell.RuneHLine, nil, styleBorder)
		}
	}
	for y := 1; y < h-1; y++ {
		s.SetContent(0, y, tcell.RuneVLine, nil, styleBorder)
		s.SetContent(mid, y, tcell.RuneVLine, nil, styleBorder)
		s.SetContent(w-1, y, tcell.RuneVLine, nil, styleBorder)
	}
	hline(0, 1, w-2)
	hline(sepY, 1, w-2)
	hline(h-1, 1, w-2)
	if l.memH > 0 {
		hline(planSepY, mid+1, w-2)
		s.SetContent(mid, planSepY, tcell.RuneLTee, nil, styleBorder)
		s.SetContent(w-1, planSepY, tcell.RuneRTee, nil, styleBorder)
	}

	s.SetContent(0, 0, tcell.RuneULCorner, nil, styleBorder)
	s.SetContent(mid, 0, tcell.RuneTTee, nil, styleBorder)
	s.SetContent(w-1, 0, tcell.RuneURCorner, nil, styleBorder)
	s.SetContent(0, sepY, tcell.RuneLTee, nil, styleBorder)
	s.SetContent(mid, sepY, tcell.RunePlus, nil, styleBorder)
	s.SetContent(w-1, sepY, tcell.RuneRTee, nil, styleBorder)
	s.SetContent(0, h-1, tcell.RuneLLCorner, nil, styleBorder)
	s.SetContent(mid, h-1, tcell.RuneBTee, nil, styleBorder)
	s.SetContent(w-1, h-1, tcell.RuneLRCorner, nil, styleBorder)
}

func strWidth(s string) int {
	return uniseg.StringWidth(s)
}

// wrap переносит текст по графемам; ширина каждой строки ≤ width
// (кроме одиночного кластера шире width).
func wrap(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = wrapLine(line, width, out)
	}
	return out
}

func wrapLine(s string, width int, out []string) []string {
	for {
		pos, cur, spStart, spEnd, state := 0, 0, -1, -1, -1
		cut, next := -1, -1
	scan:
		for pos < len(s) {
			g, _, gw, st := uniseg.FirstGraphemeClusterInString(s[pos:], state)
			state = st
			if cur+gw > width {
				switch {
				case g == " " && pos > 0:
					// Не влез сам пробел — переносим по нему.
					cut, next = pos, pos+1
				case spStart > 0:
					cut, next = spStart, spEnd
				case pos == 0:
					cut, next = len(g), len(g)
				default:
					cut, next = pos, pos
				}
				break scan
			}
			if g == " " && pos > 0 {
				spStart, spEnd = pos, pos+1
			}
			cur += gw
			pos += len(g)
		}
		if cut < 0 {
			return append(out, s)
		}
		out = append(out, s[:cut])
		s = s[next:]
		if s == "" {
			return out
		}
	}
}

// truncate обрезает справа с «…».
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if strWidth(s) <= w {
		return s
	}
	var b strings.Builder
	cur, state := 0, -1
	for s != "" {
		var g string
		var gw int
		g, s, gw, state = uniseg.FirstGraphemeClusterInString(s, state)
		if cur+gw > w-1 {
			break
		}
		b.WriteString(g)
		cur += gw
	}
	return b.String() + "…"
}

// truncateLeft обрезает слева с «…».
func truncateLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	total := strWidth(s)
	if total <= w {
		return s
	}
	state := -1
	for s != "" && total > w-1 {
		var gw int
		_, s, gw, state = uniseg.FirstGraphemeClusterInString(s, state)
		total -= gw
	}
	return "…" + s
}
