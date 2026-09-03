package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/agent2host/agent2host/internal/source/jsonreader"
	pathpkg "github.com/agent2host/agent2host/internal/source/path"
	"github.com/agent2host/agent2host/internal/space/fsync"
)

// Record is one registered Agent System (spec §4.3 semantics).
type Record struct {
	ActiveRevision string   `json:"active_revision"`
	Source         string   `json:"source"`
	Version        string   `json:"version"`
	Agents         []string `json:"agents"`
}

// Document is the on-disk registry.json object.
type Document struct {
	Systems map[string]Record `json:"systems"`
}

// Registry is the Agent Space registry under <home>/space/.
type Registry struct {
	Home     string
	mu       sync.Mutex
	sys      sync.Map // hex(SHA-256(system_id)) → *sync.Mutex
	writeErr error
}

// New resolves home to an absolute path. It does not create directories.
func New(home string) (*Registry, error) {
	abs, err := pathpkg.ResolveRoot(home)
	if err != nil {
		return nil, err
	}
	return &Registry{Home: abs}, nil
}

func (r *Registry) InjectWriteError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeErr = err
}

func (r *Registry) spaceDir() string {
	return filepath.Join(r.Home, "space")
}

func (r *Registry) jsonPath() string {
	return filepath.Join(r.spaceDir(), "registry.json")
}

func (r *Registry) registryLockPath() string {
	return filepath.Join(r.spaceDir(), "registry.lock")
}

func (r *Registry) systemLockPath(systemID string) string {
	sum := sha256.Sum256([]byte(systemID))
	return filepath.Join(r.spaceDir(), "systems", hex.EncodeToString(sum[:])+".lock")
}

func (r *Registry) systemMu(systemID string) *sync.Mutex {
	key := r.systemLockPath(systemID)
	v, _ := r.sys.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// LockSystem acquires the per-system operation lock (in-process + flock).
// Call unlock when the operation is finished. Do not hold the registry write
// lock across Artifact build; acquire that separately via WithWrite.
func (r *Registry) LockSystem(systemID string) (unlock func() error, err error) {
	mu := r.systemMu(systemID)
	mu.Lock()
	lk, err := acquire(r.systemLockPath(systemID))
	if err != nil {
		mu.Unlock()
		return nil, err
	}
	return func() error {
		err := lk.Unlock()
		mu.Unlock()
		return err
	}, nil
}

// Load reads registry.json without the write lock. Atomic rename makes a
// complete old or new file visible; this is not a substitute for WithWrite.
func (r *Registry) Load() (*Document, error) {
	return r.read()
}

// Get returns the record for systemID, or KindUnknown if absent.
func (r *Registry) Get(systemID string) (Record, error) {
	doc, err := r.read()
	if err != nil {
		return Record{}, err
	}
	rec, ok := doc.Systems[systemID]
	if !ok {
		return Record{}, fail(KindUnknown, systemID)
	}
	return rec, nil
}

// WithWrite holds the registry write lock, re-reads, runs fn, and atomically
// replaces registry.json. Must not be used to hold the lock during Artifact build.
func (r *Registry) WithWrite(fn func(*Document) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	lk, err := acquire(r.registryLockPath())
	if err != nil {
		return err
	}
	defer func() { _ = lk.Unlock() }()
	doc, err := r.read()
	if err != nil {
		return err
	}
	if err := fn(doc); err != nil {
		return err
	}
	return r.write(doc)
}

func (r *Registry) read() (*Document, error) {
	raw, err := os.ReadFile(r.jsonPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Document{Systems: map[string]Record{}}, nil
		}
		return nil, err
	}
	if _, err := jsonreader.Read(raw); err != nil {
		return nil, fail(KindCorrupt, err.Error())
	}
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fail(KindCorrupt, err.Error())
	}
	if doc.Systems == nil {
		doc.Systems = map[string]Record{}
	}
	for id, rec := range doc.Systems {
		if err := validRecord(id, rec); err != nil {
			return nil, err
		}
	}
	return &doc, nil
}

func validRecord(id string, rec Record) error {
	if rec.ActiveRevision == "" || rec.Source == "" || rec.Version == "" || len(rec.Agents) == 0 {
		return fail(KindCorrupt, id)
	}
	if id == "" || strings.ContainsAny(id, `/\`) {
		return fail(KindCorrupt, id)
	}
	if !validRevision(rec.ActiveRevision) {
		return fail(KindCorrupt, id)
	}
	return nil
}

func validRevision(rev string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(rev, prefix) {
		return false
	}
	hex := rev[len(prefix):]
	if len(hex) != 64 {
		return false
	}
	for i := 0; i < len(hex); i++ {
		c := hex[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func (r *Registry) write(doc *Document) error {
	if err := r.writeErr; err != nil {
		r.writeErr = nil
		return err
	}
	dir := r.spaceDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(dir, ".registry-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, r.jsonPath()); err != nil {
		return err
	}
	committed = true
	fsync.SyncCommittedDirBestEffort(dir)
	return nil
}
