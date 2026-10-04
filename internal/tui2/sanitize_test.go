package tui2

import (
	"math/rand"
	"strings"
	"testing"
	"unicode"
)

func TestSanitize(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "Hello, world! 123 ~[]{}", "Hello, world! 123 ~[]{}"},
		{"cyrillic", "Привет, мир! Ёё ъ", "Привет, мир! Ёё ъ"},
		{"cjk emoji combining", "你好 👍🏽 e\u0301 👨\u200d👩\u200d👧", "你好 👍🏽 e\u0301 👨\u200d👩\u200d👧"},
		{"empty", "", ""},

		{"csi color", "a\x1b[31mred\x1b[0m", "ared"},
		{"csi params", "x\x1b[1;2;3Hy\x1b[?25lz", "xyz"},
		{"csi intermediate", "x\x1b[1 qy", "xy"},
		{"osc bel", "x\x1b]0;title\x07y", "xy"},
		{"osc st", "x\x1b]8;;http://e\x1b\\link\x1b]8;;\x1b\\y", "xlinky"},
		{"osc st c1", "x\x1b]0;t\u009cy", "xy"},
		{"dcs st", "x\x1bPq#0;2;0;0;0\x1b\\y", "xy"},
		{"dcs not closed by bel", "x\x1bPabc\x07def", "x"},
		{"sos", "x\x1bXsos\x1b\\y", "xy"},
		{"pm", "x\x1b^pm\x1b\\y", "xy"},
		{"apc", "x\x1b_apc\x1b\\y", "xy"},
		{"string esc esc st", "x\x1b]0;a\x1b\x1b\\y", "xy"},
		{"other esc", "a\x1bcb\x1b7c", "abc"},

		{"c1 csi", "x\u009b31my", "xy"},
		{"c1 osc bel", "x\u009d0;t\u0007y", "xy"},
		{"c1 osc st", "x\u009d0;t\u009cy", "xy"},
		{"c1 dcs", "x\u0090abc\u009cy", "xy"},
		{"c1 sos", "x\u0098abc\x1b\\y", "xy"},
		{"c1 pm", "x\u009eabc\u009cy", "xy"},
		{"c1 apc", "x\u009fabc\u009cy", "xy"},
		{"lone st", "x\u009cy", "xy"},

		{"unclosed csi", "abc\x1b[31", "abc"},
		{"unclosed osc", "abc\x1b]0;title\nnext", "abc"},
		{"unclosed dcs", "abc\x1bPxx", "abc"},
		{"lone esc", "abc\x1b", "abc"},
		{"unclosed c1 csi", "abc\u009b", "abc"},
		{"unclosed c1 osc", "abc\u009dtitle", "abc"},

		{"bidi", "a\u202Ab\u202Bc\u202Cd\u202De\u202Ef\u2066g\u2067h\u2068i\u2069j", "abcdefghij"},

		{"crlf", "a\r\nb", "a\nb"},
		{"cr", "a\rb", "a\nb"},
		{"cr cr", "a\r\rb", "a\n\nb"},
		{"lf cr", "a\n\rb", "a\n\nb"},
		{"lf", "a\nb\n", "a\nb\n"},
		{"tab", "a\tb", "a    b"},
		{"controls", "a\x00b\x07c\x08d\x7fe\u0085f\x0bg\x0ch", "abcdefgh"},

		{"invalid utf8", "a\xffb", "a\uFFFDb"},
		{"invalid utf8 seq", "a\xc3\x28b", "a\uFFFD(b"},
		{"invalid in csi", "a\x1b[\xff31mb", "ab"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitize(c.in); got != c.want {
				t.Fatalf("sanitize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// Свойство: что бы ни пришло на вход, на выходе нет управляющих символов
// (кроме \n), ESC/C1 и bidi-оверрайдов, а строка — валидный UTF-8.
func TestSanitizeRandom(t *testing.T) {
	rnd := rand.New(rand.NewSource(42))
	alphabet := []string{"\x1b", "[", "]", "P", "\\", "\x07", "\u009b", "\u009c", "\u009d", "\u0090",
		"\r", "\n", "\t", "a", "я", "m", "0", ";", "\xff", "\u202e", "\u2066", "\x00", "👍", " "}
	for i := 0; i < 5000; i++ {
		var b strings.Builder
		for j := rnd.Intn(30); j > 0; j-- {
			b.WriteString(alphabet[rnd.Intn(len(alphabet))])
		}
		in := b.String()
		out := sanitize(in)
		if !strings.ContainsRune(out, '\uFFFD') && strings.ToValidUTF8(out, "") != out {
			t.Fatalf("invalid utf8 in %q -> %q", in, out)
		}
		for _, r := range out {
			if r == '\n' {
				continue
			}
			if unicode.IsControl(r) || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
				t.Fatalf("sanitize(%q) = %q contains %U", in, out, r)
			}
		}
		if sanitize(out) != out {
			t.Fatalf("sanitize is not idempotent: %q -> %q -> %q", in, out, sanitize(out))
		}
	}
}

func TestTokens(t *testing.T) {
	cases := map[int64]string{
		0:           "0 tokens",
		5:           "5 tokens",
		500:         "500 tokens",
		1536:        "1.5K tokens",
		1023 * 1024: "1023K tokens",
		3 << 20:     "3.0M tokens",
		1 << 31:     "2048M tokens",
		1 << 40:     "1048576M tokens",
	}
	for in, want := range cases {
		if got := tokens(in); got != want {
			t.Errorf("tokens(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestSafeChoice(t *testing.T) {
	cases := []struct {
		opts []string
		want int
	}{
		{[]string{"Yes", "No"}, 1},
		{[]string{"Allow", "Deny", "Allow all"}, 1},
		{[]string{"Да", "Нет"}, 1},
		{[]string{"Cancel", "OK"}, 0},
		{[]string{"A", "B", "C"}, 2},
		{[]string{"only"}, 0},
	}
	for _, c := range cases {
		if got := safeChoice(c.opts); got != c.want {
			t.Errorf("safeChoice(%q) = %d, want %d", c.opts, got, c.want)
		}
	}
}

