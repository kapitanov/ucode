package difftool

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$`)
var simpleDiffHeaderRe = regexp.MustCompile(`^\(\+\d+ -\d+\)$`)

// isUnifiedDiff returns true if the diff uses standard unified diff format.
func isUnifiedDiff(diff string) bool {
	for _, line := range strings.SplitN(diff, "\n", 10) {
		if strings.HasPrefix(line, "--- ") || hunkHeaderRe.MatchString(line) {
			return true
		}
	}
	return false
}

// applySimpleDiff applies a simple diff format:
//
//	(+N -N)        — optional header, ignored
//	- removed line — line to find and remove (content after "- ")
//	+ added line   — line to insert (content after "+ ")
//	 context line  — context (space-prefixed), used to locate nearby changes
//
// Removal lines are first matched by content-after-prefix; if not found, the
// entire diff line (including the "- " prefix) is tried, which handles
// markdown list items that start with "- ".
func applySimpleDiff(source, diff string) (string, error) {
	type block struct {
		remove []string // content to find and remove
		add    []string // content to insert
	}

	lines := strings.Split(diff, "\n")
	srcLines := strings.Split(source, "\n")

	var blocks []block
	var cur block
	inBlock := false

	flush := func() {
		if inBlock {
			blocks = append(blocks, cur)
			cur = block{}
			inBlock = false
		}
	}

	for _, l := range lines {
		switch {
		case simpleDiffHeaderRe.MatchString(strings.TrimSpace(l)):
			// skip header
		case strings.HasPrefix(l, "- ") && !strings.HasPrefix(l, "--- "):
			cur.remove = append(cur.remove, l[2:]) // strip "- "
			inBlock = true
		case strings.HasPrefix(l, "+ ") && !strings.HasPrefix(l, "+++ "):
			cur.add = append(cur.add, l[2:]) // strip "+ "
			inBlock = true
		case strings.HasPrefix(l, "-") && len(l) > 1 && l[1] != '-':
			cur.remove = append(cur.remove, l[1:]) // strip "-"
			inBlock = true
		case strings.HasPrefix(l, "+") && len(l) > 1 && l[1] != '+':
			cur.add = append(cur.add, l[1:]) // strip "+"
			inBlock = true
		default:
			flush()
		}
	}
	flush()

	for _, b := range blocks {
		if len(b.remove) == 0 {
			continue
		}
		idx := findConsecutiveLines(srcLines, b.remove)
		if idx < 0 {
			// Fallback: try matching the full line including "- " prefix
			withPrefix := make([]string, len(b.remove))
			for i, r := range b.remove {
				withPrefix[i] = "- " + r
			}
			idx = findConsecutiveLines(srcLines, withPrefix)
			if idx < 0 {
				return "", fmt.Errorf("could not find lines to remove: %q", strings.Join(b.remove, "\n"))
			}
			b.remove = withPrefix
		}
		next := make([]string, 0, len(srcLines)-len(b.remove)+len(b.add))
		next = append(next, srcLines[:idx]...)
		next = append(next, b.add...)
		next = append(next, srcLines[idx+len(b.remove):]...)
		srcLines = next
	}

	return strings.Join(srcLines, "\n"), nil
}

// findConsecutiveLines returns the index of the first occurrence of needle as
// a consecutive sequence in haystack, or -1 if not found.
func findConsecutiveLines(haystack, needle []string) int {
	if len(needle) == 0 {
		return -1
	}
outer:
	for i := 0; i <= len(haystack)-len(needle); i++ {
		for j, l := range needle {
			if haystack[i+j] != l {
				continue outer
			}
		}
		return i
	}
	return -1
}

// fixHunkCounts rewrites @@ headers with correct line counts, since LLMs
// often generate wrong counts which cause BSD patch to hit "unexpected EOF".
func fixHunkCounts(diff string) string {
	lines := strings.Split(diff, "\n")
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		m := hunkHeaderRe.FindStringSubmatch(lines[i])
		if m == nil {
			out = append(out, lines[i])
			i++
			continue
		}
		fromLine, toLine, suffix := m[1], m[2], m[3]
		i++
		start := i
		var fromCount, toCount int
		for i < len(lines) {
			l := lines[i]
			// stop at next hunk header or file header
			if hunkHeaderRe.MatchString(l) || strings.HasPrefix(l, "--- ") || strings.HasPrefix(l, "+++ ") {
				break
			}
			// trailing empty line is the artifact of strings.Split, not a hunk line
			if l == "" && i == len(lines)-1 {
				break
			}
			switch {
			case strings.HasPrefix(l, "+"):
				toCount++
			case strings.HasPrefix(l, "-"):
				fromCount++
			default: // context line (space-prefixed or blank)
				fromCount++
				toCount++
			}
			i++
		}
		out = append(out, fmt.Sprintf("@@ -%s,%d +%s,%d @@%s", fromLine, fromCount, toLine, toCount, suffix))
		out = append(out, lines[start:i]...)
	}
	return strings.Join(out, "\n")
}

type DiffPreview struct {
	Lines          []string
	Added, Removed int
}

func Preview(diff string) DiffPreview {
	var preview DiffPreview
	for _, line := range strings.Split(diff, "\n") {
		preview.Lines = append(preview.Lines, line)
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++") {
			preview.Added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "--") {
			preview.Removed++
		}
	}

	return preview
}

func Apply(source, diff string) (string, error) {
	if !isUnifiedDiff(diff) {
		return applySimpleDiff(source, diff)
	}

	srcFile, err := os.CreateTemp("", "ucode-patch-src-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(srcFile.Name())

	if _, err = srcFile.WriteString(source); err != nil {
		srcFile.Close()
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	srcFile.Close()

	diffFile, err := os.CreateTemp("", "ucode-patch-diff-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp diff file: %w", err)
	}
	defer os.Remove(diffFile.Name())

	if !strings.HasSuffix(diff, "\n") {
		diff += "\n"
	}
	diff = fixHunkCounts(diff)

	if _, err = diffFile.WriteString(diff); err != nil {
		diffFile.Close()
		return "", fmt.Errorf("failed to write temp diff file: %w", err)
	}
	diffFile.Close()

	cmd := exec.Command("patch", "-s", "-l", srcFile.Name(), diffFile.Name())
	defer os.Remove(srcFile.Name() + ".orig") // BSD patch creates a backup; clean it up
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("patch failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	result, err := os.ReadFile(srcFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read patched file: %w", err)
	}
	return string(result), nil
}
