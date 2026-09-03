package rule

import (
	"errors"
	"fmt"

	"github.com/agent2host/agent2host/internal/source/jsonreader"
)

// Error is a Source-contract failure. ID is a frozen SRC-* rule id.
type Error struct {
	ID     string
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.ID
	}
	return fmt.Sprintf("%s: %s", e.ID, e.Detail)
}

func Fail(id, detail string) *Error {
	return &Error{ID: id, Detail: detail}
}

// Warning is a register warning and must not fail register.
type Warning struct {
	ID     string `json:"id"`
	Detail string `json:"detail"`
}

// ID extracts a frozen SRC-* id from err, if any.
func ID(err error) string {
	if err == nil {
		return ""
	}
	var r *Error
	if errors.As(err, &r) {
		return r.ID
	}
	var j *jsonreader.Error
	if errors.As(err, &j) {
		return j.RuleID
	}
	return ""
}
