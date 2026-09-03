package jsonreader

import "fmt"

// Error is a json_reader failure. RuleID is a frozen SRC-JSON-* id.
type Error struct {
	RuleID string
	Offset int
	Detail string
}

// LimitError is an implementation resource limit (for example encoding/json
// nesting). It is not SRC-JSON-SYNTAX and is not part of v1alpha1.
type LimitError struct {
	Offset int
	Detail string
}

func (e *LimitError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("jsonreader: resource limit at byte %d", e.Offset)
	}
	return fmt.Sprintf("jsonreader: resource limit at byte %d: %s", e.Offset, e.Detail)
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s at byte %d", e.RuleID, e.Offset)
	}
	return fmt.Sprintf("%s at byte %d: %s", e.RuleID, e.Offset, e.Detail)
}

func utf8Err(offset int, detail string) *Error {
	return &Error{RuleID: "SRC-JSON-UTF8", Offset: offset, Detail: detail}
}

func syntaxErr(offset int, detail string) *Error {
	return &Error{RuleID: "SRC-JSON-SYNTAX", Offset: offset, Detail: detail}
}

func dupErr(offset int, key string) *Error {
	return &Error{RuleID: "SRC-JSON-DUP", Offset: offset, Detail: fmt.Sprintf("duplicate object key %q", key)}
}
