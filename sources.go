package livepkg

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
)

type Sources struct {
	// Dir is the root directory for the sources
	Dir string

	init   sync.Once
	loaded atomic.Value
}

func NewSources(dir string) *Sources {
	return &Sources{Dir: dir}
}

// returns the cached state of files
func (s *Sources) Files() (files []*File, err error) {
	s.init.Do(func() { err = s.Reload() })
	if err != nil {
		return nil, err
	}

	files = s.loaded.Load().([]*File)
	return files, nil
}

// reloads the files from disk
func (s *Sources) Reload() error {
	files, err := LoadFiles(s.Dir)
	sorted, err2 := sortFiles(files)
	s.loaded.Store(sorted)

	if err != nil {
		return err
	}
	return err2
}

type Change struct {
	Prev *File `json:"prev"` // nil if file was added
	Next *File `json:"next"` // nil if file was deleted
	Deps bool  `json:"deps"` // true if dependencies changed
}

// walks the directory and returns changes compared to prev reloaded state
func (s *Sources) Changes(prevstate []*File) ([]*File, []*Change, error) {
	type marking struct {
		file  *File
		found bool
	}

	loaded := make(map[string]*marking, len(prevstate))
	for _, file := range prevstate {
		loaded[file.Name] = &marking{file: file, found: false}
	}

	changes := []*Change{}
	err := filepath.Walk(s.Dir, func(filename string, info os.FileInfo, err error) error {
		if err != nil || !supportedExt[filepath.Ext(filename)] {
			return err
		}

		track, ok := loaded[filename]
		if !ok {
			file, err := LoadFile(s.Dir, filename)
			if err != nil {
				return err
			}

			loaded[file.Name] = &marking{file, true}
			changes = append(changes, &Change{
				Prev: nil,
				Next: file,
				Deps: len(file.Deps) > 0,
			})
			return nil
		}
		track.found = true

		if !track.file.ModTime.Before(info.ModTime()) {
			return nil
		}

		next, err := track.file.Reload()
		if err != nil {
			return err
		}

		if bytes.Equal(next.Content, track.file.Content) {
			return nil
		}

		changes = append(changes, &Change{
			Prev: track.file,
			Next: next,
			Deps: !depsEqual(track.file.Deps, next.Deps),
		})
		track.file = next

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	for filename, track := range loaded {
		if !track.found {
			changes = append(changes, &Change{
				Prev: track.file,
				Next: nil,
				Deps: true,
			})
			delete(loaded, filename)
		}
	}

	if len(changes) == 0 {
		return prevstate, nil, nil
	}

	files := make([]*File, 0, len(loaded))
	for _, track := range loaded {
		files = append(files, track.file)
	}
	files, err = sortFiles(files)

	s.loaded.Store(files)
	return files, changes, err
}

func depsEqual(a, b []string) bool {
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

func (s *Sources) FileByPath(path string) (*File, error) {
	files, err := s.Files()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.Path == path {
			return file, nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *Sources) FilesByExt(ext string) ([]*File, error) {
	files, err := s.Files()
	if err != nil {
		return nil, err
	}

	byext := []*File{}
	for _, file := range files {
		if file.Ext == ext {
			byext = append(byext, file)
		}
	}
	return byext, nil
}

func (s *Sources) BundleByExt(ext string) ([]byte, error) {
	files, err := s.FilesByExt(ext)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for _, file := range files {
		fmt.Fprintf(&buf, "\n/* \"%s\" */\n", file.Path)
		buf.Write(file.Content)
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

func (s *Sources) ServeFile(w http.ResponseWriter, r *http.Request) {
	if !supportedExt[path.Ext(r.URL.Path)] {
		filename := urlfile(s.Dir, r.URL.Path)
		http.ServeFile(w, r, filename)
		return
	}

	file, err := s.FileByPath(r.URL.Path)
	if err == os.ErrNotExist {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Write(file.Content)
}
