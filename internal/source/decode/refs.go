package decode

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SkillRef is skills[] short (string id) or long {id, required?}.
type SkillRef struct {
	ID       string
	Required *bool
	short    bool
}

// ToolAllowlistEntry is tools[] short (string name) or long {name, required?}.
type ToolAllowlistEntry struct {
	Name     string
	Required *bool
	short    bool
}

func (s *SkillRef) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var id string
		if err := json.Unmarshal(b, &id); err != nil {
			return err
		}
		*s = SkillRef{ID: id, short: true}
		return nil
	}
	var aux struct {
		ID       string `json:"id"`
		Required *bool  `json:"required"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*s = SkillRef{ID: aux.ID, Required: aux.Required, short: false}
	return nil
}

func (s SkillRef) MarshalJSON() ([]byte, error) {
	if s.short {
		return json.Marshal(s.ID)
	}
	aux := struct {
		ID       string `json:"id"`
		Required *bool  `json:"required,omitempty"`
	}{ID: s.ID, Required: s.Required}
	return json.Marshal(aux)
}

// ExpandShort records that this ref is the long form (used by normalize).
func (s *SkillRef) ExpandShort() {
	s.short = false
}

// IsShort reports authoring short form.
func (s SkillRef) IsShort() bool { return s.short }

func (t *ToolAllowlistEntry) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var name string
		if err := json.Unmarshal(b, &name); err != nil {
			return err
		}
		*t = ToolAllowlistEntry{Name: name, short: true}
		return nil
	}
	var aux struct {
		Name     string `json:"name"`
		Required *bool  `json:"required"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*t = ToolAllowlistEntry{Name: aux.Name, Required: aux.Required, short: false}
	return nil
}

func (t ToolAllowlistEntry) MarshalJSON() ([]byte, error) {
	if t.short {
		return json.Marshal(t.Name)
	}
	aux := struct {
		Name     string `json:"name"`
		Required *bool  `json:"required,omitempty"`
	}{Name: t.Name, Required: t.Required}
	return json.Marshal(aux)
}

func (t *ToolAllowlistEntry) ExpandShort() {
	t.short = false
}

func (t ToolAllowlistEntry) IsShort() bool { return t.short }

func decodeBytes(raw []byte, dest any) error {
	// Unknown core fields are a Schema gate (SP-030). Do not DisallowUnknownFields:
	// a lagging struct must not reject Schema-valid input.
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
