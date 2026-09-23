package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Version of the progress file schema.
const schemaVersion = 1

// MaxRecent caps the recent-books list.
const MaxRecent = 10

// Progress is the saved reading state for one book, keyed by content hash.
type Progress struct {
	Fingerprint string    `json:"fingerprint"`
	Kind        string    `json:"kind,omitempty"`
	Path        string    `json:"path"`
	Title       string    `json:"title"`
	Creator     string    `json:"creator"`
	WordIndex   int       `json:"wordIndex"`
	WPM         int       `json:"wpm"`
	TOCHref     string    `json:"tocHref,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type db struct {
	Version int                 `json:"version"`
	Books   map[string]Progress `json:"books"`
	Recent  []string            `json:"recent"`
}

// Store persists per-book progress as versioned JSON with atomic writes.
type Store struct {
	dir  string
	path string
	data db
}

// DefaultDir returns the platform-local data directory for progress.
func DefaultDir() string {
	if cfg, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfg, "rsvp-reader")
	}
	return filepath.Join(os.TempDir(), "rsvp-reader")
}

// Open loads (or initializes) the store in dir, creating it as needed.
func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir: %w", err)
	}
	s := &Store{dir: dir, path: filepath.Join(dir, "progress.json")}
	s.data = db{Version: schemaVersion, Books: map[string]Progress{}}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("store: read: %w", err)
	}
	var loaded db
	if err := json.Unmarshal(raw, &loaded); err != nil {
		// Corrupt file: start fresh rather than crash; the old file is
		// left for inspection under a backup name.
		_ = os.WriteFile(s.path+".corrupt", raw, 0o644)
		return s, nil
	}
	if loaded.Books != nil {
		s.data.Books = loaded.Books
	}
	s.data.Recent = loaded.Recent
	return s, nil
}

// Save records progress, refreshes recency, and writes atomically.
func (s *Store) Save(p Progress) error {
	if p.Fingerprint == "" {
		return fmt.Errorf("store: missing fingerprint")
	}
	p.UpdatedAt = time.Now()
	s.data.Books[p.Fingerprint] = p
	// Move fingerprint to front of recents.
	keep := make([]string, 0, len(s.data.Recent)+1)
	keep = append(keep, p.Fingerprint)
	for _, fp := range s.data.Recent {
		if fp != p.Fingerprint {
			keep = append(keep, fp)
		}
	}
	if len(keep) > MaxRecent {
		keep = keep[:MaxRecent]
		// Drop books that fell off the recent list.
		live := map[string]bool{}
		for _, fp := range keep {
			live[fp] = true
		}
		for fp := range s.data.Books {
			if !live[fp] {
				delete(s.data.Books, fp)
			}
		}
	}
	s.data.Recent = keep
	return s.write()
}

// Lookup returns saved progress for a fingerprint.
func (s *Store) Lookup(fingerprint string) (Progress, bool) {
	p, ok := s.data.Books[fingerprint]
	return p, ok
}

// ListRecent returns recent progress entries, newest first.
func (s *Store) ListRecent() []Progress {
	var out []Progress
	for _, fp := range s.data.Recent {
		if p, ok := s.data.Books[fp]; ok {
			out = append(out, p)
		}
	}
	return out
}

// Remove deletes a fingerprint from books and recents.
func (s *Store) Remove(fingerprint string) error {
	delete(s.data.Books, fingerprint)
	keep := s.data.Recent[:0]
	for _, fp := range s.data.Recent {
		if fp != fingerprint {
			keep = append(keep, fp)
		}
	}
	s.data.Recent = keep
	return s.write()
}

// MaxTextBytes caps stored pasted-text sessions.
const MaxTextBytes = 2 << 20

// SaveText stores pasted-text content by fingerprint (atomic write) so
// text sessions resume and reopen like files.
func (s *Store) SaveText(fingerprint, text string) error {
	if len(text) > MaxTextBytes {
		return fmt.Errorf("store: pasted text exceeds %d bytes", MaxTextBytes)
	}
	dir := filepath.Join(s.dir, "texts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir texts: %w", err)
	}
	tmp := filepath.Join(dir, fingerprint+".tmp")
	final := filepath.Join(dir, fingerprint+".txt")
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return fmt.Errorf("store: write text: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("store: commit text: %w", err)
	}
	return nil
}

// LoadText returns stored pasted-text content.
func (s *Store) LoadText(fingerprint string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(s.dir, "texts", fingerprint+".txt"))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (s *Store) write() error {
	s.data.Version = schemaVersion
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("store: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}
