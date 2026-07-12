package difftool

import (
	"strings"
	"testing"
)

func TestFixHunkCounts(t *testing.T) {
	diff := "--- a/f\n+++ b/f\n@@ -1,6 +1,26 @@\n context1\n context2\n \n+added1\n+added2\n context3\n"
	fixed := fixHunkCounts(diff)
	if !strings.Contains(fixed, "@@ -1,4 +1,6 @@") {
		t.Errorf("unexpected header:\n%s", fixed)
	}
}

func TestApplyUnified(t *testing.T) {
	source := "> Search\n```\n\n## License\n\nMIT\n"
	diff := "--- a/f\n+++ b/f\n@@ -1,6 +1,26 @@\n > Search\n ```\n \n+## New\n+\n+Content\n+\n ## License\n \n MIT\n"
	result, err := Apply(source, diff)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "## New") {
		t.Errorf("new section not found:\n%s", result)
	}
}

func TestApplySimple_StripPrefix(t *testing.T) {
	source := "line1\nold line\nline3\n"
	diff := "(+1 -1)\n- old line\n+ new line\n"
	result, err := Apply(source, diff)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "new line") || strings.Contains(result, "old line") {
		t.Errorf("unexpected result:\n%s", result)
	}
}

func TestApplySimple_MarkdownListFallback(t *testing.T) {
	// Source lines ARE markdown list items starting with "- "
	source := "## Usage\n\n- **Type your prompt** and press Enter\n- **`exit`** - Quit\n- **`/plan`** - View plan\n"
	diff := "(+1 -3)\n- **Type your prompt** and press Enter\n- **`exit`** - Quit\n- **`/plan`** - View plan\n+ See README for usage.\n"
	result, err := Apply(source, diff)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "See README for usage.") {
		t.Errorf("replacement not found:\n%s", result)
	}
	if strings.Contains(result, "Type your prompt") {
		t.Errorf("old lines still present:\n%s", result)
	}
}
