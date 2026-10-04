package tui2

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestWrap(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"ascii fits", "hello world", 20, []string{"hello world"}},
		{"ascii wrap at space", "hello world foo", 11, []string{"hello world", "foo"}},
		{"ascii wrap", "aaa bbb ccc", 5, []string{"aaa", "bbb", "ccc"}},
		{"long word", "abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"cyrillic", "привет мир", 6, []string{"привет", "мир"}},
		{"cjk", "你好世界", 5, []string{"你好", "世界"}},
		{"cjk exact", "你好世界", 4, []string{"你好", "世界"}},
		{"emoji", "👍👍👍", 4, []string{"👍👍", "👍"}},
		{"combining", "e\u0301e\u0301e\u0301", 2, []string{"e\u0301e\u0301", "e\u0301"}},
		{"empty", "", 10, []string{""}},
		{"empty lines", "a\n\nb", 10, []string{"a", "", "b"}},
		{"trailing newline", "a\n", 10, []string{"a", ""}},
		{"wide wider than width", "你", 1, []string{"你"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wrap(c.text, c.width); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("wrap(%q, %d) = %q, want %q", c.text, c.width, got, c.want)
			}
		})
	}
}

// Свойства: ширина строки ≤ width (кроме одиночного кластера шире width),
// текст не теряется (без учёта пробелов, съеденных на переносе).
func TestWrapProperties(t *testing.T) {
	texts := []string{
		"The quick brown fox jumps over the lazy dog",
		"Съешь же ещё этих мягких французских булок, да выпей чаю",
		"中文文本没有空格应该硬换行中文文本没有空格",
		"emoji 👍🏽👨‍👩‍👧🇷🇺 mixed with text and 你好 and e\u0301\u0302 combining",
		"supercalifragilisticexpialidocious_without_any_spaces_at_all",
		"  leading spaces and   multiple   spaces  ",
		"line1\nline2 is longer than the others\n\nline4",
		"a 你 b 好 c 👍 d",
	}
	for _, text := range texts {
		for width := 1; width <= 30; width++ {
			lines := wrap(text, width)
			if len(lines) < strings.Count(text, "\n")+1 {
				t.Fatalf("wrap(%q, %d): lost lines: %q", text, width, lines)
			}
			for _, l := range lines {
				if w := strWidth(l); w > width && !singleCluster(l) {
					t.Fatalf("wrap(%q, %d): line %q has width %d", text, width, l, w)
				}
			}
			norm := func(s string) string { return strings.NewReplacer(" ", "", "\n", "").Replace(s) }
			if norm(strings.Join(lines, "")) != norm(text) {
				t.Fatalf("wrap(%q, %d) lost text: %q", text, width, lines)
			}
		}
	}
}

func singleCluster(s string) bool {
	n := 0
	state := -1
	for s != "" {
		_, s, _, state = uniseg.FirstGraphemeClusterInString(s, state)
		n++
	}
	return n == 1
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate fits: %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Errorf("truncate: %q", got)
	}
	if got := truncate("你好世界", 4); strWidth(got) > 4 {
		t.Errorf("truncate wide: %q", got)
	}
	if got := truncateLeft("/home/user/project", 8); got != "…project" {
		t.Errorf("truncateLeft: %q", got)
	}
	if got := truncateLeft("/дом/проект", 20); got != "/дом/проект" {
		t.Errorf("truncateLeft fits: %q", got)
	}
	for w := 0; w < 12; w++ {
		if got := truncateLeft("/你好/世界/👍", w); strWidth(got) > w {
			t.Errorf("truncateLeft(%d) = %q too wide", w, got)
		}
		if got := truncate("/你好/世界/👍", w); strWidth(got) > w {
			t.Errorf("truncate(%d) = %q too wide", w, got)
		}
	}
}

func TestComputeLayout(t *testing.T) {
	sizes := [][2]int{{80, 24}, {40, 10}, {200, 60}, {41, 11}, {120, 30}, {300, 100}, {57, 13}}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			l := computeLayout(w, h)
			if l.tooSmall {
				t.Fatalf("unexpected tooSmall")
			}
			if l.input.h != 3 {
				t.Errorf("input height %d, want 3", l.input.h)
			}
			if l.usage.h != 1 {
				t.Errorf("usage height %d, want 1", l.usage.h)
			}
			if l.rightW < 24 || l.rightW > 50 {
				t.Errorf("rightW %d out of [24,50]", l.rightW)
			}
			if l.usage.w != l.plan.w {
				t.Errorf("usage width %d != plan width %d", l.usage.w, l.plan.w)
			}
			rects := map[string]rect{"chat": l.chat, "plan": l.plan, "input": l.input, "usage": l.usage}
			if l.memH > 0 {
				rects["memory"] = l.memory
			}
			for name, r := range rects {
				if r.w < 1 || r.h < 1 {
					t.Errorf("%s is empty: %+v", name, r)
				}
				// Внутренние области не залезают на внешнюю рамку.
				if r.x < 1 || r.y < 1 || r.x+r.w > w-1 || r.y+r.h > h-1 {
					t.Errorf("%s %+v out of screen %dx%d", name, r, w, h)
				}
			}
			for n1, r1 := range rects {
				for n2, r2 := range rects {
					if n1 < n2 && overlap(r1, r2) {
						t.Errorf("%s %+v overlaps %s %+v", n1, r1, n2, r2)
					}
				}
			}
			// Каждая ячейка — не более одной панели; рамки между ними свободны.
			mid := l.leftW + 1
			for y := 0; y < h; y++ {
				for _, x := range []int{0, mid, w - 1} {
					if _, ok := l.panelAt(x, y); ok {
						t.Errorf("border cell (%d,%d) belongs to a panel", x, y)
					}
				}
			}
			for x := 0; x < w; x++ {
				for _, y := range []int{0, h - 5, h - 1} {
					if _, ok := l.panelAt(x, y); ok {
						t.Errorf("border cell (%d,%d) belongs to a panel", x, y)
					}
				}
			}
			if l.input.y+l.input.h != h-1 || l.input.y != h-4 {
				t.Errorf("input %+v not at bottom", l.input)
			}
			if l.chat.y+l.chat.h != h-5 {
				t.Errorf("chat %+v must end at separator y=%d", l.chat, h-5)
			}
		})
	}
}

func TestComputeLayoutTooSmall(t *testing.T) {
	for _, sz := range [][2]int{{39, 24}, {80, 9}, {10, 3}, {0, 0}, {1, 1}, {-5, -5}} {
		if l := computeLayout(sz[0], sz[1]); !l.tooSmall {
			t.Errorf("computeLayout(%d,%d) must be tooSmall", sz[0], sz[1])
		}
		if _, ok := computeLayout(sz[0], sz[1]).panelAt(0, 0); ok {
			t.Errorf("panelAt on tooSmall layout must be false")
		}
	}
}

func overlap(a, b rect) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

