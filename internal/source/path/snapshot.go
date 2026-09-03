package path

import "fmt"

// Snapshot is one register's validated file bytes. After Load, later stages
// consume only this map — they must not re-read Source.
type Snapshot struct {
	fs    *FS
	files map[string][]byte
}

// NewSnapshot starts an empty per-register byte snapshot.
func NewSnapshot(fs *FS) *Snapshot {
	return &Snapshot{fs: fs, files: map[string][]byte{}}
}

// Load runs CheckDeclared once per canonical path and returns a copy of the
// stored bytes. A second Load of the same path does not touch the filesystem.
func (s *Snapshot) Load(canonical string, role Role) ([]byte, error) {
	if raw, ok := s.files[canonical]; ok {
		return append([]byte(nil), raw...), nil
	}
	raw, err := s.fs.CheckDeclared(canonical, role)
	if err != nil {
		return nil, err
	}
	s.files[canonical] = append([]byte(nil), raw...)
	return append([]byte(nil), raw...), nil
}

// Bytes returns a copy of already-loaded bytes. It never reads Source.
func (s *Snapshot) Bytes(canonical string) ([]byte, error) {
	raw, ok := s.files[canonical]
	if !ok {
		return nil, fmt.Errorf("snapshot: %s was not loaded in this register", canonical)
	}
	return append([]byte(nil), raw...), nil
}
