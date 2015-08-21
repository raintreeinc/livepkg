package livepkg

import (
	"bytes"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Errors is a wrapper for multiple errors
type Errors []error

// Error is for implementing error interface
func (errs Errors) Error() string {
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "\n")
}

// Nilify returns nil, if there are no errors
func (errs Errors) Nilify() error {
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// Bundle is a collection of source files that can be bundled and/or reloaded
type Bundle struct {
	// Root is the filesystem used to load/reload sources
	Root http.FileSystem
	// Main is the root files that are used to find rest of the files
	Main []string

	// sources contains the list of reloaded files
	sources atomic.Value
}

// NewBundle returns a empty bundle
func NewBundle(root http.FileSystem, main ...string) *Bundle {
	bundle := &Bundle{
		Root: root,
		Main: main,
	}
	bundle.sources.Store([]*Source{})
	return bundle
}

// Change shows info about a source file change
type Change struct {
	Prev *Source `json:"prev"` // Prev is nil if file was added
	Next *Source `json:"next"` // Next is nil if file was deleted
	Deps bool    `json:"deps"` // Deps is true if dependencies changed
}

// current returns the state after the last reload
func (b *Bundle) current() []*Source { return b.sources.Load().([]*Source) }

// Reload reloads the content from Root and returns the list of changes and
// all errors that occurred
func (b *Bundle) Reload() ([]*Change, error) {
	var errs Errors

	current := b.current()

	track := make(map[string]*Change, len(current))
	unchecked := append([]string{}, b.Main...)
	for _, src := range current {
		track[src.Path] = &Change{Prev: src}
		unchecked = append(unchecked, src.Path)
	}

	checked := make(map[string]bool)
	for len(unchecked) > 0 {
		path := unchecked[len(unchecked)-1]
		unchecked = unchecked[:len(unchecked)-1]
		if checked[path] {
			continue
		}
		checked[path] = true

		info, ok := track[path]
		if !ok {
			source, err := b.Load(path)
			if err != nil && err != ErrUnknownImport {
				errs = append(errs, err)
				continue
			}

			track[path] = &Change{
				Next: source,
				Deps: len(source.Deps) > 0,
			}

			for _, dep := range source.Deps {
				if !checked[dep] {
					unchecked = append(unchecked, dep)
				}
			}
			continue
		}

		changed, next, err := b.ReloadSource(info.Prev)
		if next == nil {
			continue
		}
		if err != nil {
			errs = append(errs, err)
		}

		for _, dep := range next.Deps {
			if !checked[dep] {
				unchecked = append(unchecked, dep)
			}
		}

		if changed {
			info.Next = next
			info.Deps = !sameDeps(info.Prev.Deps, next.Deps)
		} else {
			info.Next = info.Prev
		}
	}

	sources := []*Source{}
	changes := []*Change{}
	for _, info := range track {
		if info.Next != nil {
			sources = append(sources, info.Next)
		}

		if info.Next != info.Prev || info.Deps {
			changes = append(changes, info)
		}
	}
	if len(changes) == 0 {
		return []*Change{}, errs.Nilify()
	}

	sorted, err := SortSources(sources)
	if err != nil {
		errs = append(errs, err)
	}

	b.sources.Store(sorted)

	return changes, errs.Nilify()
}

// Load loads a source from path
func (b *Bundle) Load(path string) (*Source, error) {
	_, next, err := b.ReloadSource(&Source{Path: path})
	return next, err
}

// ReloadSource reloads the base file and returns a new Source file in next.
// If file doesn't exist any more it will return nil as next
func (b *Bundle) ReloadSource(prev *Source) (changed bool, next *Source, err error) {
	file, err := b.Root.Open(prev.Path)
	if err != nil {
		return true, nil, err
	}
	defer file.Close()

	ext := filepath.Ext(prev.Path)
	next = &Source{
		Path: prev.Path,
		Ext:  ext,

		ContentType: mime.TypeByExtension(ext),
	}

	stat, staterr := file.Stat()
	if staterr == nil {
		next.ModTime = stat.ModTime()

		if next.ModTime.Equal(prev.ModTime) {
			next.Content = prev.Content
			next.Processed = prev.Processed
			return false, next, nil
		}
	} else {
		next.ModTime = time.Now()
	}
	if err = next.ReadFrom(file); err != nil {
		return true, next, err
	}

	return !bytes.Equal(prev.Content, next.Content), next, nil
}

// All returns the list of sorted sources
// Do not modify this list!
func (b *Bundle) All() []*Source {
	cur := b.current()
	if cur == nil {
		return []*Source{}
	}
	return cur
}

// All returns sorted sources with specified ext
// Do not modify this list!
func (b *Bundle) ByExt(ext string) []*Source {
	byext := []*Source{}
	for _, src := range b.current() {
		if src.Ext == ext {
			byext = append(byext, src)
		}
	}
	return byext
}

// MergedByExt bundles files together into bytes by ext
func (b *Bundle) MergedByExt(ext string) []byte {
	var buf bytes.Buffer
	for _, src := range b.ByExt(ext) {
		fmt.Fprintf(&buf, "\n/* \"%s\" */\n", src.Path)
		buf.Write(src.Processed)
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

// fromCache loads path from cache if it exists, otherwise loads from Root
func (b *Bundle) fromCache(path string) (*Source, error) {
	for _, src := range b.current() {
		if src.Path == path {
			return src, nil
		}
	}

	return b.Load(path)
}

// ServeFile serves file from cache or Root
func (b *Bundle) ServeFile(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
		r.URL.Path = upath
	}

	src, err := b.fromCache(upath)
	if err == os.ErrNotExist {
		http.NotFound(w, r)
		return
	}

	if err != nil && err != ErrUnknownImport {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", src.ContentType)
	w.Write(src.Processed)
}

// sameDeps returns true if the dependencies are the same
func sameDeps(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
