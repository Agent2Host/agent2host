package registry

import (
	"fmt"
)

// Kind is an internal registry failure. These are not SRC-* rule ids.
type Kind string

const (
	KindBusy       Kind = "busy"
	KindProvenance Kind = "provenance"
	KindCorrupt    Kind = "corrupt"
	KindUnknown    Kind = "unknown"
)

// Error is a typed registry failure.
type Error struct {
	Kind   Kind
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return "registry: " + string(e.Kind)
	}
	return fmt.Sprintf("registry: %s: %s", e.Kind, e.Detail)
}

func fail(k Kind, detail string) *Error {
	return &Error{Kind: k, Detail: detail}
}
