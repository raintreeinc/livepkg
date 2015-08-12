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
)

type Errors []error

func (errs Errors) Error() string {
	var msgs []string
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "\n")
}

func (errs Errors) Nilify() error {
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Bundle struct {
	Root http.FileSystem
	Main []string

	sources atomic.Value
}

func NewBundle(root http.FileSystem, main ...string) *Bundle {
	bundle := &Bundle{
		Root: root,
		Main: main,
	}
	bundle.sources.Store([]*Source{})
	return bundle
}

type Change struct {
	Prev *Source `json:"prev"` // nil if file was added
	Next *Source `json:"next"` // nil if file was deleted
	Deps bool    `json:"deps"` // true if dependencies changed
}

func (b *Bundle) current() []*Source {
	return b.sources.Load().([]*Source)
}
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

		if changed {
			info.Next = next
			info.Deps = !sameDeps(info.Prev.Deps, next.Deps)

			for _, dep := range next.Deps {
				if !checked[dep] {
					unchecked = append(unchecked, dep)
				}
			}
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

func (b *Bundle) Load(path string) (*Source, error) {
	_, next, err := b.ReloadSource(&Source{Path: path})
	return next, err
}

func (b *Bundle) ReloadSource(base *Source) (changed bool, next *Source, err error) {
	file, err := b.Root.Open(base.Path)
	if err != nil {
		return true, nil, err
	}
	defer file.Close()

	ext := filepath.Ext(base.Path)
	next = &Source{
		Path: base.Path,
		Ext:  ext,

		ContentType: mime.TypeByExtension(ext),
	}

	stat, staterr := file.Stat()
	if staterr == nil {
		next.ModTime = stat.ModTime()
		if !base.ModTime.Before(stat.ModTime()) {
			next.Content = base.Content
			next.Processed = base.Processed
			return false, next, nil
		}
	}

	if err = next.ReadFrom(file); err != nil {
		return true, next, err
	}

	return !bytes.Equal(base.Content, next.Content), next, nil
}

func (b *Bundle) All() []*Source {
	cur := b.current()
	if cur == nil {
		return []*Source{}
	}
	return cur
}

func (b *Bundle) ByExt(ext string) []*Source {
	byext := []*Source{}
	for _, src := range b.current() {
		if src.Ext == ext {
			byext = append(byext, src)
		}
	}
	return byext
}

func (b *Bundle) MergedByExt(ext string) []byte {
	var buf bytes.Buffer
	for _, src := range b.ByExt(ext) {
		fmt.Fprintf(&buf, "\n/* \"%s\" */\n", src.Path)
		buf.Write(src.Processed)
		buf.WriteByte('\n')
	}

	return buf.Bytes()
}

func (b *Bundle) fromCache(path string) (*Source, error) {
	for _, src := range b.current() {
		if src.Path == path {
			return src, nil
		}
	}

	return b.Load(path)
}

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
