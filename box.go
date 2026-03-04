package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// ─────────────────────────────────────────────
//  Box — structured session storage
// ─────────────────────────────────────────────

// Box is the thread-safe in-memory session store.
type Box struct {
	mu      sync.Mutex
	entries []*BoxEntry
	counter int
}

// NewBox creates an empty Box.
func NewBox() *Box {
	return &Box{}
}

// autoKey generates a unique key like "out_4".
func (b *Box) autoKey() string {
	return fmt.Sprintf("out_%d", b.counter+1)
}

// StoreTable stores a table result under the given key.
// If key == "", an auto-key is used.
func (b *Box) StoreTable(key, source string, cols []string, rows []Row) *BoxEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.counter++
	if key == "" {
		key = fmt.Sprintf("out_%d", b.counter)
	}
	// Copy rows to avoid mutation
	cp := make([]Row, len(rows))
	for i, r := range rows {
		nr := make(Row, len(r))
		for k, v := range r {
			nr[k] = v
		}
		cp[i] = nr
	}
	e := &BoxEntry{
		ID:      b.counter,
		Key:     key,
		Type:    TypeTable,
		Cols:    cols,
		Rows:    cp,
		Source:  source,
		Created: time.Now(),
		Updated: time.Now(),
	}
	b.entries = append(b.entries, e)
	return e
}

// StoreText stores plain-text output.
func (b *Box) StoreText(key, source, text string) *BoxEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.counter++
	if key == "" {
		key = fmt.Sprintf("out_%d", b.counter)
	}
	e := &BoxEntry{
		ID:      b.counter,
		Key:     key,
		Type:    TypeText,
		Text:    text,
		Source:  source,
		Created: time.Now(),
		Updated: time.Now(),
	}
	b.entries = append(b.entries, e)
	return e
}

// Get retrieves an entry by key name or numeric ID string.
// When multiple entries share a key, the most recent is returned.
func (b *Box) Get(keyOrID string) (*BoxEntry, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Try numeric ID
	var id int
	if n, err := fmt.Sscan(keyOrID, &id); n == 1 && err == nil {
		for _, e := range b.entries {
			if e.ID == id {
				return e, true
			}
		}
	}
	// Try key name (last match)
	for i := len(b.entries) - 1; i >= 0; i-- {
		if b.entries[i].Key == keyOrID {
			return b.entries[i], true
		}
	}
	return nil, false
}

// Remove deletes an entry by key or ID. Returns how many were removed.
func (b *Box) Remove(keyOrID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	var id int
	byID := false
	if n, err := fmt.Sscan(keyOrID, &id); n == 1 && err == nil {
		byID = true
	}
	before := len(b.entries)
	out := b.entries[:0]
	for _, e := range b.entries {
		if byID && e.ID == id {
			continue
		}
		if !byID && e.Key == keyOrID {
			continue
		}
		out = append(out, e)
	}
	b.entries = out
	return before - len(b.entries)
}

// Rename renames a box entry.
func (b *Box) Rename(oldKey, newKey string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range b.entries {
		if e.Key == oldKey {
			e.Key = newKey
			e.Updated = time.Now()
			return true
		}
	}
	return false
}

// Tag adds a tag to an entry.
func (b *Box) Tag(keyOrID, tag string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.findLocked(keyOrID)
	if e == nil {
		return false
	}
	for _, t := range e.Tags {
		if t == tag {
			return true // already tagged
		}
	}
	e.Tags = append(e.Tags, tag)
	e.Updated = time.Now()
	return true
}

// Untag removes a tag from an entry.
func (b *Box) Untag(keyOrID, tag string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.findLocked(keyOrID)
	if e == nil {
		return false
	}
	tags := e.Tags[:0]
	for _, t := range e.Tags {
		if t != tag {
			tags = append(tags, t)
		}
	}
	e.Tags = tags
	e.Updated = time.Now()
	return true
}

// Clear removes all entries.
func (b *Box) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = nil
}

// List returns entries matching an optional search string and/or tag filter.
// Pass "" for search/tag to skip that filter.
func (b *Box) List(search, tag string) []*BoxEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []*BoxEntry
	for _, e := range b.entries {
		if search != "" {
			searchLow := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(e.Key), searchLow) &&
				!strings.Contains(strings.ToLower(e.Source), searchLow) {
				continue
			}
		}
		if tag != "" {
			found := false
			for _, t := range e.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

// Keys returns all unique key names currently in the box (for tab-completion).
func (b *Box) Keys() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	seen := make(map[string]bool)
	var out []string
	for _, e := range b.entries {
		if !seen[e.Key] {
			seen[e.Key] = true
			out = append(out, e.Key)
		}
	}
	sort.Strings(out)
	return out
}

// Len returns the number of entries.
func (b *Box) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.entries)
}

// ExportJSON writes all box entries to a file as JSON.
func (b *Box) ExportJSON(path string) error {
	b.mu.Lock()
	data, err := json.MarshalIndent(b.entries, "", "  ")
	b.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ImportJSON reads box entries from a JSON file and appends them.
func (b *Box) ImportJSON(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var entries []*BoxEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range entries {
		b.counter++
		e.ID = b.counter
		b.entries = append(b.entries, e)
	}
	return len(entries), nil
}

// findLocked finds an entry by key or id without locking (must hold mu).
func (b *Box) findLocked(keyOrID string) *BoxEntry {
	var id int
	if n, err := fmt.Sscan(keyOrID, &id); n == 1 && err == nil {
		for _, e := range b.entries {
			if e.ID == id {
				return e
			}
		}
	}
	for i := len(b.entries) - 1; i >= 0; i-- {
		if b.entries[i].Key == keyOrID {
			return b.entries[i]
		}
	}
	return nil
}
