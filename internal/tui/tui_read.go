package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
)

var cursorStyle = color.New(color.FgHiGreen, color.Bold).SprintFunc()

func NewReader() *Reader {
	return &Reader{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

type Reader struct {
	scanner *bufio.Scanner
}

func (r *Reader) Next() (string, bool) {
	rl, err := readline.New(fmt.Sprintf("%s ", cursorStyle(">")))
	if err != nil {
		panic(err)
	}
	defer func() { _ = rl.Close() }()

	str, err := rl.Readline()
	if err != nil { // io.EOF
		return "", false
	}

	str = strings.TrimSpace(str)

	if str == "" {
		return "", true
	}

	return str, true
}
