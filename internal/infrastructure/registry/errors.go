package registry

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrAllSourcesFailed indicates all configured sources failed
	ErrAllSourcesFailed = errors.New("all registry sources failed")

	// ErrInvalidFormat indicates the registry data format is invalid
	ErrInvalidFormat = errors.New("invalid registry data format")

	// ErrEmptyData indicates no data was received from source
	ErrEmptyData = errors.New("empty registry data received")

	// ErrParsingFailed indicates registry data parsing failed
	ErrParsingFailed = errors.New("failed to parse registry data")

	// ErrUnsupportedFormat indicates the data format is not supported
	ErrUnsupportedFormat = errors.New("unsupported registry data format")
)

// SourceError wraps errors from registry sources with additional context
type SourceError struct {
	Source    string
	Operation string
	Cause     error
	Timestamp time.Time
}

func (e *SourceError) Error() string {
	return fmt.Sprintf("registry source %q failed during %s: %v (at %s)",
		e.Source, e.Operation, e.Cause, e.Timestamp.Format(time.RFC3339))
}

func (e *SourceError) Unwrap() error {
	return e.Cause
}

// NewSourceError creates a new SourceError with current timestamp
func NewSourceError(source, operation string, cause error) *SourceError {
	return &SourceError{
		Source:    source,
		Operation: operation,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// ParsingError wraps errors that occur during registry data parsing
type ParsingError struct {
	Format string
	Line   int
	Column int
	Cause  error
}

func (e *ParsingError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("parsing %s format failed at line %d, column %d: %v",
			e.Format, e.Line, e.Column, e.Cause)
	}
	return fmt.Sprintf("parsing %s format failed: %v", e.Format, e.Cause)
}

func (e *ParsingError) Unwrap() error {
	return e.Cause
}

// NewParsingError creates a new ParsingError
func NewParsingError(format string, cause error) *ParsingError {
	return &ParsingError{
		Format: format,
		Cause:  cause,
	}
}

// NewParsingErrorWithPosition creates a new ParsingError with line/column info
func NewParsingErrorWithPosition(format string, line, column int, cause error) *ParsingError {
	return &ParsingError{
		Format: format,
		Line:   line,
		Column: column,
		Cause:  cause,
	}
}
