package ui

import (
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps briandowns/spinner with our color scheme
type Spinner struct {
	spinner *spinner.Spinner
}

// NewSpinner creates a new spinner with a message
func NewSpinner(message string) *Spinner {
	s := spinner.New(
		spinner.CharSets[14], // dots style
		100*time.Millisecond,
		spinner.WithColor("cyan"),
		spinner.WithSuffix(" "+message),
	)
	return &Spinner{spinner: s}
}

// Start starts the spinner
func (s *Spinner) Start() {
	s.spinner.Start()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.spinner.Stop()
}

// Success stops the spinner and shows a success message
func (s *Spinner) Success(message string) {
	s.spinner.Stop()
	_, _ = successColor.Printf("%s %s\n", successSymbol, message)
}

// Error stops the spinner and shows an error message
func (s *Spinner) Error(message string) {
	s.spinner.Stop()
	_, _ = errorColor.Printf("%s %s\n", errorSymbol, message)
}

// Warning stops the spinner and shows a warning message
func (s *Spinner) Warning(message string) {
	s.spinner.Stop()
	_, _ = warningColor.Printf("%s %s\n", warningSymbol, message)
}

// UpdateMessage updates the spinner message while it's running
func (s *Spinner) UpdateMessage(message string) {
	s.spinner.Suffix = " " + message
}

// WithSpinner runs a function with a spinner
// Returns the spinner so you can call Success/Error on it
func WithSpinner(message string, fn func() error) error {
	s := NewSpinner(message)
	s.Start()

	err := fn()

	if err != nil {
		s.Error(message + " failed")
		return err
	}

	s.Success(message)
	return nil
}
