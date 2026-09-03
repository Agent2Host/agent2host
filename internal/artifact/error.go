package artifact

import "fmt"

// Kind is an internal Artifact integrity failure. These are not SRC-* rule ids.
type Kind string

const (
	KindDigestFormat      Kind = "digest_format"
	KindEmptyFiles        Kind = "empty_files"
	KindDuplicatePath     Kind = "duplicate_path"
	KindMemberType        Kind = "member_type"
	KindSystemJSONCount   Kind = "system_json_count"
	KindNoncanonicalOrder Kind = "noncanonical_order"
	KindNoncanonicalPath  Kind = "noncanonical_path"
)

// Error is a typed Artifact-manifest failure.
type Error struct {
	Kind   Kind
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return "artifact: " + string(e.Kind)
	}
	return fmt.Sprintf("artifact: %s: %s", e.Kind, e.Detail)
}

func fail(k Kind, detail string) *Error {
	return &Error{Kind: k, Detail: detail}
}
