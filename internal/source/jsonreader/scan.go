package jsonreader

import "encoding/json"

// scanJSON rejects duplicate keys (SRC-JSON-DUP) and illegal JSON (SRC-JSON-SYNTAX).
// It is iterative: it does not impose an Agent2Host nesting limit and does not
// map resource exhaustion onto SRC-JSON-SYNTAX.
func scanJSON(b []byte) error {
	s := scanner{b: b}
	if err := s.parseValue(); err != nil {
		return err
	}
	s.skipWS()
	if s.i != len(s.b) {
		return syntaxErr(s.i, "trailing data after top-level value")
	}
	return nil
}

type scanner struct {
	b []byte
	i int
}

type containerKind uint8

const (
	kindObject containerKind = iota
	kindArray
)

type frame struct {
	kind       containerKind
	seen       map[string]struct{}
	afterValue bool
	expectKey  bool
	empty      bool
}

func (s *scanner) skipWS() {
	for s.i < len(s.b) {
		c := s.b[s.i]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			s.i++
			continue
		}
		break
	}
}

func (s *scanner) parseValue() error {
	s.skipWS()
	if s.i >= len(s.b) {
		return syntaxErr(s.i, "unexpected end of input")
	}
	switch s.b[s.i] {
	case '{':
		s.i++
		return s.parseContainers([]frame{{kind: kindObject, seen: map[string]struct{}{}, expectKey: true, empty: true}})
	case '[':
		s.i++
		return s.parseContainers([]frame{{kind: kindArray, empty: true}})
	default:
		return s.parseLeaf()
	}
}

func (s *scanner) parseContainers(stack []frame) error {
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		s.skipWS()
		if s.i >= len(s.b) {
			if top.kind == kindObject {
				return syntaxErr(s.i, "unterminated object")
			}
			return syntaxErr(s.i, "unterminated array")
		}
		c := s.b[s.i]

		if top.kind == kindObject {
			if !top.afterValue && top.expectKey {
				if c == '}' {
					if len(top.seen) != 0 {
						return syntaxErr(s.i, "expected object key")
					}
					s.i++
					stack = stack[:len(stack)-1]
					if len(stack) > 0 {
						stack[len(stack)-1].afterValue = true
					}
					continue
				}
				if c != '"' {
					return syntaxErr(s.i, "expected object key")
				}
				keyOff := s.i
				key, err := s.parseString()
				if err != nil {
					return err
				}
				if _, ok := top.seen[key]; ok {
					return dupErr(keyOff, key)
				}
				top.seen[key] = struct{}{}
				s.skipWS()
				if s.i >= len(s.b) || s.b[s.i] != ':' {
					return syntaxErr(s.i, "expected ':'")
				}
				s.i++
				top.expectKey = false
				continue
			}
			if !top.afterValue {
				n, err := s.openOrLeaf()
				if err != nil {
					return err
				}
				if n != nil {
					top.empty = false
					stack = append(stack, *n)
					continue
				}
				top.empty = false
				top.afterValue = true
				continue
			}
			switch c {
			case ',':
				s.i++
				top.afterValue = false
				top.expectKey = true
			case '}':
				s.i++
				stack = stack[:len(stack)-1]
				if len(stack) > 0 {
					stack[len(stack)-1].afterValue = true
				}
			default:
				return syntaxErr(s.i, "expected ',' or '}'")
			}
			continue
		}

		// array
		if !top.afterValue {
			if c == ']' {
				if !top.empty {
					return syntaxErr(s.i, "expected array value")
				}
				s.i++
				stack = stack[:len(stack)-1]
				if len(stack) > 0 {
					stack[len(stack)-1].afterValue = true
				}
				continue
			}
			n, err := s.openOrLeaf()
			if err != nil {
				return err
			}
			if n != nil {
				top.empty = false
				stack = append(stack, *n)
				continue
			}
			top.empty = false
			top.afterValue = true
			continue
		}
		switch c {
		case ',':
			s.i++
			top.afterValue = false
		case ']':
			s.i++
			stack = stack[:len(stack)-1]
			if len(stack) > 0 {
				stack[len(stack)-1].afterValue = true
			}
		default:
			return syntaxErr(s.i, "expected ',' or ']'")
		}
	}
	return nil
}

func (s *scanner) openOrLeaf() (*frame, error) {
	s.skipWS()
	if s.i >= len(s.b) {
		return nil, syntaxErr(s.i, "unexpected end of input")
	}
	switch s.b[s.i] {
	case '{':
		s.i++
		return &frame{kind: kindObject, seen: map[string]struct{}{}, expectKey: true, empty: true}, nil
	case '[':
		s.i++
		return &frame{kind: kindArray, empty: true}, nil
	default:
		return nil, s.parseLeaf()
	}
}

func (s *scanner) parseLeaf() error {
	s.skipWS()
	if s.i >= len(s.b) {
		return syntaxErr(s.i, "unexpected end of input")
	}
	switch s.b[s.i] {
	case '"':
		_, err := s.parseString()
		return err
	case 't':
		return s.expectLiteral("true")
	case 'f':
		return s.expectLiteral("false")
	case 'n':
		return s.expectLiteral("null")
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return s.parseNumber()
	default:
		return syntaxErr(s.i, "unexpected character")
	}
}

func (s *scanner) parseString() (string, error) {
	if s.i >= len(s.b) || s.b[s.i] != '"' {
		return "", syntaxErr(s.i, "expected string")
	}
	start := s.i
	s.i++
	for s.i < len(s.b) {
		c := s.b[s.i]
		if c == '\\' {
			s.i++
			if s.i >= len(s.b) {
				return "", syntaxErr(s.i, "unterminated string escape")
			}
			s.i++
			continue
		}
		if c == '"' {
			s.i++
			var out string
			if err := json.Unmarshal(s.b[start:s.i], &out); err != nil {
				return "", syntaxErr(start, "invalid string")
			}
			return out, nil
		}
		if c < 0x20 {
			return "", syntaxErr(s.i, "unescaped control character")
		}
		s.i++
	}
	return "", syntaxErr(start, "unterminated string")
}

func (s *scanner) expectLiteral(lit string) error {
	if s.i+len(lit) > len(s.b) || string(s.b[s.i:s.i+len(lit)]) != lit {
		return syntaxErr(s.i, "invalid literal")
	}
	s.i += len(lit)
	return nil
}

func (s *scanner) parseNumber() error {
	start := s.i
	if s.b[s.i] == '-' {
		s.i++
	}
	if s.i >= len(s.b) {
		return syntaxErr(start, "invalid number")
	}
	if s.b[s.i] == '0' {
		s.i++
	} else if s.b[s.i] >= '1' && s.b[s.i] <= '9' {
		for s.i < len(s.b) && s.b[s.i] >= '0' && s.b[s.i] <= '9' {
			s.i++
		}
	} else {
		return syntaxErr(start, "invalid number")
	}
	if s.i < len(s.b) && s.b[s.i] == '.' {
		s.i++
		if s.i >= len(s.b) || s.b[s.i] < '0' || s.b[s.i] > '9' {
			return syntaxErr(start, "invalid number")
		}
		for s.i < len(s.b) && s.b[s.i] >= '0' && s.b[s.i] <= '9' {
			s.i++
		}
	}
	if s.i < len(s.b) && (s.b[s.i] == 'e' || s.b[s.i] == 'E') {
		s.i++
		if s.i < len(s.b) && (s.b[s.i] == '+' || s.b[s.i] == '-') {
			s.i++
		}
		if s.i >= len(s.b) || s.b[s.i] < '0' || s.b[s.i] > '9' {
			return syntaxErr(start, "invalid number")
		}
		for s.i < len(s.b) && s.b[s.i] >= '0' && s.b[s.i] <= '9' {
			s.i++
		}
	}
	return nil
}
