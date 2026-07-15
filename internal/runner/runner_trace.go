package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type tracer struct {
	path, name string
}

func newTracer(name string) *tracer {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	path := filepath.Join(wd, ".agents", "trace.txt")
	return &tracer{path: path, name: name}
}

func (t *tracer) New(name string) *tracer {
	return &tracer{path: t.path, name: name}
}

func (t *tracer) Prompt(message string) {
	t.writePrefixed(fmt.Sprintf("%s <<< ", t.name), message)
}

func (t *tracer) Reasoning(message string) {
	t.writePrefixed(fmt.Sprintf("%s (REASONING) >>> ", t.name), message)
}

func (t *tracer) Response(message string) {
	t.writePrefixed(fmt.Sprintf("%s >>> ", t.name), message)
}

func (t *tracer) Refusal(message string) {
	t.writePrefixed(fmt.Sprintf("%s (REFUSAL) >>> ", t.name), message)
}

func (t *tracer) Ask(question string, options []string) {
	t.writePrefixed(fmt.Sprintf("%s (ASK) >>> ", t.name), fmt.Sprintf("%s | [%s]", question, strings.Join(options, ",")))
}

func (t *tracer) Answer(answer string) {
	t.writePrefixed(fmt.Sprintf("%s (ANSWER) <<< ", t.name), answer)
}

func (t *tracer) ToolCall(name, args string) {
	t.writePrefixed(fmt.Sprintf("%s: ", t.name), fmt.Sprintf("%s(%s)", name, args))
}

func (t *tracer) ToolCallResult(result string) {
	t.writePrefixed("    ", result)
}

func (t *tracer) writePrefixed(prefix, text string) {
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		str := fmt.Sprintf("%s%s\n", prefix, line)
		sb.WriteString(str)
	}

	t.write(sb.String())
}

func (t *tracer) write(text string) {
	err := os.MkdirAll(filepath.Dir(t.path), 0755)
	if err != nil {
		return
	}

	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}

	defer func() { _ = f.Close() }()

	_, _ = f.WriteString(text)
}
