package tui

import (
	"time"

	"github.com/briandowns/spinner"
)

type Spinner interface {
	Pause(f func())
}

func WithSpinner(f func(Spinner)) {
	s := spinner.New(spinner.CharSets[57], 100*time.Millisecond)
	s.Start()
	defer s.Stop()

	f(&spinnerImpl{spinner: s})
}

type spinnerImpl struct {
	spinner *spinner.Spinner
}

func (s *spinnerImpl) Pause(f func()) {
	s.spinner.Stop()
	defer s.spinner.Start()

	f()
}
