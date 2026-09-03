package store

import (
	"errors"
	"fmt"
)

// Kind is an internal Artifact Store failure. These are not SRC-* rule ids.
type Kind string

const (
	KindCorrupt  Kind = "corrupt"
	KindMissing  Kind = "missing"
	KindRevision Kind = "revision"
)

// Error is a typed store failure.
type Error struct {
	Kind   Kind
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return "store: " + string(e.Kind)
	}
	return fmt.Sprintf("store: %s: %s", e.Kind, e.Detail)
}

func fail(k Kind, detail string) *Error {
	return &Error{Kind: k, Detail: detail}
}

func asCorrupt(err error) error {
	if err == nil {
		return nil
	}
	var se *Error
	if errors.As(err, &se) {
		return err
	}
	return fail(KindCorrupt, err.Error())
}
