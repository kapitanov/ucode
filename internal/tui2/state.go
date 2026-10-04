package tui2

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/kapitanov/ucode/internal/iface"
)

type panel int

const (
	panelInput panel = iota
	panelChat
	panelPlan
	panelMemory
	panelCount
)

type entryKind int

const (
	kindText entryKind = iota
	kindToolRunning
	kindToolOK
	kindToolFail
)

type segment struct {
	text  string
	style tcell.Style
}

// chatEntry — запись чата; wrapped кэширует перенос для ширины wrappedW.
type chatEntry struct {
	kind    entryKind
	prefix  []segment
	text    string
	style   tcell.Style
	detail  string
	prefixW int

	wrapped       []string
	wrappedDetail []string
	wrappedW      int
}

func (e *chatEntry) lineCount() int {
	return len(e.wrapped) + len(e.wrappedDetail)
}

type statusItem struct {
	id    int
	text  string
	until time.Time
}

type askState struct {
	question string
	options  []string
	selected int
	answered bool
	ch       chan int
}

var (
	styleDefault   = tcell.StyleDefault
	styleWhite     = tcell.StyleDefault.Foreground(tcell.ColorWhite)
	styleBold      = styleWhite.Bold(true)
	styleGray      = tcell.StyleDefault.Foreground(tcell.ColorGray)
	styleDim       = styleGray.Dim(true)
	styleRed       = tcell.StyleDefault.Foreground(tcell.ColorRed)
	styleGreen     = tcell.StyleDefault.Foreground(tcell.ColorLime)
	styleQuestion  = tcell.StyleDefault.Foreground(tcell.ColorFuchsia)
	stylePrintf    = tcell.StyleDefault.Foreground(tcell.ColorOlive).Italic(true)
	styleReasoning = tcell.StyleDefault.Foreground(tcell.ColorTeal).Italic(true)
	styleToolRun   = tcell.StyleDefault.Foreground(tcell.ColorOlive).Dim(true)
	styleToolOK    = tcell.StyleDefault.Foreground(tcell.ColorGreen).Dim(true)
	styleBorder    = styleGray
	styleNewLines  = tcell.StyleDefault.Foreground(tcell.ColorYellow)
)

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// Константы для ограничений чата
const (
	maxEntryRunes = 10000 // максимальная длина одной записи в рунах
	maxChatLines  = 10000 // максимальное число записей в чате
)

// sanitize вычищает управляющие и escape-последовательности из внешних строк.
// Также удаляет BOM (U+FEFF), variation selectors (U+FE00–U+FE0F),
// U+200B–U+200C, U+2060–U+206F. ZWJ (U+200D) сохраняется (тесты это требуют).
func sanitize(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	var b strings.Builder
	b.Grow(len(s))

	const (
		stNormal = iota
		stEsc
		stCSI
		stString // OSC/DCS/SOS/PM/APC до ST
		stStringEsc
	)
	state := stNormal
	osc := false
	prevCR := false

	for _, r := range s {
		switch state {
		case stEsc:
			switch r {
			case 0x1b:
				// ESC ESC — новая последовательность.
			case '[':
				state = stCSI
			case ']':
				state, osc = stString, true
			case 'P', 'X', '^', '_':
				state, osc = stString, false
			default:
				state = stNormal
			}
			cleanedSequences.Add(1)
			continue
		case stCSI:
			if r == 0x1b {
				// ESC прерывает CSI и начинает новую последовательность.
				state = stEsc
			} else if r >= 0x40 && r <= 0x7E {
				state = stNormal
			}
			cleanedSequences.Add(1)
			continue
		case stString:
			if r == 0x1b {
				state = stStringEsc
			} else if r == 0x9c || (osc && r == 0x07) {
				state = stNormal
			}
			cleanedSequences.Add(1)
			continue
		case stStringEsc:
			if r == 0x5c { // ST: ESC \				state = stNormal
			} else if r != 0x1b {
				state = stString
			}
			cleanedSequences.Add(1)
			continue
		}

		switch r {
		case 0x1b:
			state = stEsc
			continue
		case 0x9b:
			state = stCSI
			continue
		case 0x9d:
			state, osc = stString, true
			continue
		case 0x90, 0x98, 0x9e, 0x9f:
			state, osc = stString, false
			continue
		case '\r':
			b.WriteByte('\n')
			prevCR = true
			continue
		case '\n':
			if !prevCR {
				b.WriteByte('\n')
			}
			prevCR = false
			continue
		case '\t':
			b.WriteString("    ")
			prevCR = false
			continue
		}
		prevCR = false

		// Удаляем управляющие символы, bidi-форматирование, enclosure,
		// BOM (U+FEFF), variation selectors (U+FE00–U+FE0F),
		// U+200B–U+200C, U+2060–U+206F. ZWJ (U+200D) сохраняется.
		if unicode.IsControl(r) ||
			(r >= 0x202A && r <= 0x202E) ||
			(r >= 0x2066 && r <= 0x2069) ||
			r == 0xFEFF || // BOM
			(r >= 0xFE00 && r <= 0xFE0F) || // variation selectors
			(r >= 0x200B && r <= 0x200C) || // zero-width space, non-joiner
			(r >= 0x2060 && r <= 0x206F) { // word joiner, invisible operators, etc.
			cleanedSequences.Add(1)
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

// cleanedSequences — счётчик очищенных последовательностей для метрик/отладки.
var cleanedSequences atomic.Int64

// GetCleanedSequences возвращает текущее значение счётчика очищенных последовательностей.
func GetCleanedSequences() int64 {
	return cleanedSequences.Load()
}

// ResetCleanedSequences сбрасывает счётчик очищенных последовательностей.
func ResetCleanedSequences() {
	cleanedSequences.Store(0)
}

func clonePlan(p *iface.Plan) iface.Plan {
	if p == nil {
		return iface.Plan{}
	}
	items := append([]iface.PlanItem(nil), p.Items...)
	for i := range items {
		items[i].Title = sanitize(items[i].Title)
	}
	return iface.Plan{Items: items}
}

func cloneMemory(m *iface.Memory) iface.Memory {
	if m == nil {
		return iface.Memory{}
	}
	items := append([]iface.MemoryItem(nil), m.Items...)
	for i := range items {
		items[i].Key = sanitize(items[i].Key)
		items[i].Value = sanitize(items[i].Value)
	}
	return iface.Memory{Items: items}
}

// safeChoice выбирает «отказный» вариант, иначе последний.
func safeChoice(options []string) int {
	for i, o := range options {
		l := strings.ToLower(strings.TrimSpace(o))
		for _, p := range []string{"no", "deny", "reject", "cancel", "нет"} {
			if strings.HasPrefix(l, p) {
				return i
			}
		}
	}
	return len(options) - 1
}

func tokens(s int64) string {
	if s < 10 {
		return fmt.Sprintf("%d tokens", s)
	}
	symbols := []string{" tokens", "K tokens", "M tokens"}

	i := math.Floor(
		math.Log(float64(s)) / math.Log(1024),
	)
	i = min(i, float64(len(symbols)-1))
	size := float64(s) / math.Pow(1024, math.Floor(i))
	format := "%.0f"
	if size < 10 {
		format = "%.1f"
	}

	return fmt.Sprintf(format+"%s", size, symbols[int(i)])
}

func newEntry(prefix []segment, text string, style tcell.Style) *chatEntry {
	w := 0
	for _, p := range prefix {
		w += strWidth(p.text)
	}
	return &chatEntry{kind: kindText, prefix: prefix, text: text, style: style, prefixW: w}
}

// addEntry добавляет запись; вызывать под mu.
// Ограничивает длину текста записи maxEntryRunes и общее число записей maxChatLines.
func (u *UI) addEntry(e *chatEntry) {
	// Ограничиваем длину текста записи
	if utf8.RuneCountInString(e.text) > maxEntryRunes {
		e.text = string([]rune(e.text)[:maxEntryRunes])
	}

	// Ограничиваем общее число записей в чате
	if len(u.chat) >= maxChatLines {
		// Удаляем старые записи, оставляя последние maxChatLines
		removeCount := len(u.chat) - maxChatLines + 1
		u.chat = u.chat[removeCount:]
	}

	u.chat = append(u.chat, e)
	if !u.chatFollow {
		u.newWhileScrolled = true
	}
}

func (u *UI) addAgentText(agentName, text string, nameStyle, textStyle tcell.Style) {
	var prefix []segment
	if agentName != "" {
		prefix = []segment{{agentName, nameStyle}, {": ", textStyle}}
	}
	u.addEntry(newEntry(prefix, text, textStyle))
}

func planWidth(p iface.Plan) int {
	return len(fmt.Sprintf("%d", len(p.Items)))
}

func planMark(item iface.PlanItem) segment {
	if item.Done {
		return segment{"▣", styleGreen}
	}
	return segment{"◻", styleRed}
}

// addPlanToChat выводит план в чат (как printPlan старого tui); под mu.
func (u *UI) addPlanToChat() {
	p := u.plan
	u.addEntry(newEntry(nil, fmt.Sprintf("PLAN (%d/%d)", len(p.Completed()), len(p.Items)), styleBold))
	w := planWidth(p)
	for _, item := range p.Items {
		prefix := []segment{
			{fmt.Sprintf("  %*d) ", w, item.Index), styleWhite},
			planMark(item),
			{" ", styleWhite},
		}
		u.addEntry(newEntry(prefix, item.Title, styleWhite))
	}
}

func (u *UI) addMemoryToChat() {
	m := u.memory
	u.addEntry(newEntry(nil, fmt.Sprintf("MEMORY (%d)", len(m.Items)), styleBold))
	for _, item := range m.Items {
		u.addEntry(newEntry([]segment{{"  " + item.Key + ": ", styleBold}}, item.Value, styleWhite))
	}
}

// runCommand выполняет команду из поля ввода; true — выход. Под mu.
func (u *UI) runCommand(str string) bool {
	switch str {
	case "/exit", "/quit":
		return true
	case "/plan":
		u.addPlanToChat()
	case "/memory":
		u.addMemoryToChat()
	case "/usage":
		text := u.usageText
		if text == "" {
			text = "—"
		}
		u.addEntry(newEntry(nil, "USAGE: "+text, styleGray))
	default:
		u.addEntry(newEntry(nil, sanitize(fmt.Sprintf("Invalid command: %q", str)), styleRed))
	}
	u.chatFollow = true
	return false
}

// statusLine — текст строки состояния и признак спиннера; под mu.
func (u *UI) statusLine() (string, bool) {
	spin := len(u.status) > 0
	if u.quitRequested {
		return "Выход после завершения запроса...", spin
	}
	if spin {
		return u.status[len(u.status)-1].text, true
	}
	return u.baseStatus, false
}

func (u *UI) pushStatus(text string, until time.Time) int {
	u.nextID++
	u.status = append(u.status, statusItem{id: u.nextID, text: text, until: until})
	return u.nextID
}

func (u *UI) popStatus(id int) {
	for i, s := range u.status {
		if s.id == id {
			u.status = append(u.status[:i], u.status[i+1:]...)
			return
		}
	}
}

func (u *UI) expireStatus(now time.Time) {
	kept := u.status[:0]
	for _, s := range u.status {
		if s.until.IsZero() || now.Before(s.until) {
			kept = append(kept, s)
		}
	}
	u.status = kept
}

// plainTail — последние n строк чата без оформления; под mu.
func (u *UI) plainTail(n int) []string {
	var lines []string
	for _, e := range u.chat {
		var prefix string
		switch e.kind {
		case kindToolRunning:
			prefix = "… "
		case kindToolOK:
			prefix = "✓ "
		case kindToolFail:
			prefix = "✗ "
		default:
			for _, p := range e.prefix {
				prefix += p.text
			}
		}
		indent := strings.Repeat(" ", utf8.RuneCountInString(prefix))
		for i, l := range strings.Split(e.text, "\n") {
			if i == 0 {
				lines = append(lines, prefix+l)
			} else {
				lines = append(lines, indent+l)
			}
		}
		if e.detail != "" {
			for _, l := range strings.Split(e.detail, "\n") {
				lines = append(lines, indent+l)
			}
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
