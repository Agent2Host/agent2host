package space

import "fmt"

// Kind is an Agent Space library failure (not a SRC-* rule id).
type Kind string

const (
	KindUnknownAgent Kind = "unknown_agent"
	KindBadTarget    Kind = "bad_target"
	KindMismatch     Kind = "revision_mismatch"
	KindTooLarge     Kind = "inclusion_too_large"
)

// Error is a typed Space failure.
type Error struct {
	Kind   Kind
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return "space: " + string(e.Kind)
	}
	return fmt.Sprintf("space: %s: %s", e.Kind, e.Detail)
}

func fail(k Kind, detail string) *Error {
	return &Error{Kind: k, Detail: detail}
}
