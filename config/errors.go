package config

import (
	"fmt"
	"strings"
)

// Location identifies a one-based source position.
type Location struct {
	File         string
	Line, Column int
}

// IssueKind is a stable category for callers using errors.As.
type IssueKind string

// Configuration error categories.
const (
	IssueSource       IssueKind = "source"
	IssueSyntax       IssueKind = "syntax"
	IssueDuplicate    IssueKind = "duplicate"
	IssueUnknownField IssueKind = "unknown_field"
	IssueType         IssueKind = "type"
	IssueRequired     IssueKind = "required"
	IssueConstraint   IssueKind = "constraint"
	IssueCanceled     IssueKind = "canceled"
)

// Issue describes a failure without retaining raw configuration values.
type Issue struct {
	Path     string
	Source   string
	Kind     IssueKind
	Message  string
	Location *Location
}

// Error aggregates issues in schema order. Safe filesystem and context errors
// are retained for errors.Is and errors.As; conversion errors are sanitized.
type Error struct {
	Issues []Issue
	cause  error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("configuration error:")
	for _, issue := range e.Issues {
		b.WriteString("\n  ")
		if issue.Path != "" {
			b.WriteString(issue.Path + ": ")
		}
		b.WriteString(issue.Message)
		if issue.Source != "" {
			b.WriteString(" [" + issue.Source + "]")
		}
		if l := issue.Location; l != nil {
			fmt.Fprintf(&b, " %s:%d:%d", l.File, l.Line, l.Column)
		}
	}
	return b.String()
}

// Unwrap preserves safe underlying source and cancellation errors.
func (e *Error) Unwrap() error { return e.cause }

func sourceError(kind IssueKind, source, message string, cause error) *Error {
	return &Error{Issues: []Issue{{Kind: kind, Source: source, Message: message}}, cause: cause}
}
