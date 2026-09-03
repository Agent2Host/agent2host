package jsonreader

import (
	"bytes"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Document is ValidatedJSON: UTF-8, no BOM, legal JSON, no duplicate keys.
type Document struct {
	Bytes []byte
	Value any
}

// Read applies SRC-JSON-UTF8, then SRC-JSON-SYNTAX / SRC-JSON-DUP, in that order.
func Read(b []byte) (*Document, error) {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return nil, utf8Err(0, "UTF-8 BOM is illegal")
	}
	if !utf8.Valid(b) {
		return nil, utf8Err(firstInvalidUTF8(b), "not valid UTF-8")
	}
	if err := scanJSON(b); err != nil {
		return nil, err
	}
	// Second pass: materialize Value with json.Number so legal JSON numbers
	// (including 1e400 and integers beyond float64) are not SRC-JSON-SYNTAX.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		off := 0
		if se, ok := err.(*json.SyntaxError); ok {
			off = int(se.Offset)
			if off > 0 {
				off--
			}
		}
		if isDecoderLimit(err) {
			return nil, &LimitError{Offset: off, Detail: err.Error()}
		}
		return nil, syntaxErr(off, err.Error())
	}
	return &Document{Bytes: append([]byte(nil), b...), Value: v}, nil
}

func isDecoderLimit(err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "max depth") || strings.Contains(msg, "nesting") {
		return true
	}
	return false
}

func firstInvalidUTF8(b []byte) int {
	i := 0
	for i < len(b) {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			return i
		}
		i += size
	}
	return 0
}
