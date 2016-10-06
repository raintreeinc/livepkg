package livepkg

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestLoadLinear(t *testing.T) {
	fs := filesystem{
		"/main.js":   `depends("alpha.js")`,
		"/alpha.js":  `depends("/beta/x.js")`,
		"/beta/x.js": `depends("../last.js")`,
		"/last.js":   ``,
	}

	bundle := NewBundle(fs, "/main.js")
	_, err := bundle.Reload()
	if err != nil {
		t.Errorf("err %v", err)
	}

	sources := bundle.All()
	if !sameFiles(sources, []string{"/last.js", "/beta/x.js", "/alpha.js", "/main.js"}) {
		t.Errorf("got %v", names(sources))
	}
}

func TestLoadDAG(t *testing.T) {
	fs := filesystem{
		"/main.js":  `depends("alpha.js"); depends("beta.js");`,
		"/alpha.js": `depends("last.js")`,
		"/beta.js":  `depends("last.js")`,
		"/last.js":  ``,
	}

	bundle := NewBundle(fs, "/main.js")
	_, err := bundle.Reload()
	if err != nil {
		t.Errorf("err %v", err)
	}

	sources := bundle.All()
	if !(sameFiles(sources, []string{"/last.js", "/beta.js", "/alpha.js", "/main.js"}) ||
		sameFiles(sources, []string{"/last.js", "/alpha.js", "/beta.js", "/main.js"})) {
		t.Errorf("got %v", names(sources))
	}
}

func TestLoadCycle(t *testing.T) {
	fs := filesystem{
		"/a.js": `depends("b.js")`,
		"/b.js": `depends("c.js")`,
		"/c.js": `depends("a.js")`,
	}

	bundle := NewBundle(fs, "/a.js")
	_, err := bundle.Reload()
	if err == nil {
		t.Errorf("should have gotten an error for cycle")
	}
}

func TestReloadChangeFile(t *testing.T) {
	fs := filesystem{"/main.js": ``}

	bundle := NewBundle(fs, "/main.js")
	_, err := bundle.Reload()
	if err != nil {
		t.Errorf("err initial load: %v", err)
	}

	fs["/main.js"] = `CONTENT`
	changes, err := bundle.Reload()
	if err != nil {
		t.Errorf("err %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("invalid number of changes: %#v", changes)
		return
	}

	if !(changes[0].Prev != nil &&
		bytes.Equal(changes[0].Prev.Content, []byte(``)) &&
		changes[0].Next != nil &&
		bytes.Equal(changes[0].Next.Content, []byte(`CONTENT`)) &&
		changes[0].Deps == false) {
		t.Errorf("should've detected file modification")
	}

	fs["/main.js"] = `depends("/other.js")`
	fs["/other.js"] = ``

	changes, err = bundle.Reload()
	if err != nil {
		t.Errorf("err %v", err)
	}
	sort.Sort(changesByPath(changes))

	if len(changes) != 2 {
		t.Errorf("invalid number of changes after deps change: %#v", changes)
		return
	}

	if !(changes[0].Deps == true) {
		t.Errorf("should've detected file modification %v", changes)
	}

	if !(changes[1].Prev == nil && changes[1].Next != nil &&
		changes[1].Next.Path == `/other.js`) {
		t.Errorf("should've loaded new file")
	}

	delete(fs, "/other.js")

	changes, _ = bundle.Reload()

	if len(changes) != 1 {
		t.Errorf("invalid number of changes after file delete: %#v", changes)
		return
	}
	if !(changes[0].Next == nil) {
		t.Errorf("should've detected file deletion %#v", changes[0])
	}

	fs["/other.js"] = `blah`
	changes, _ = bundle.Reload()
	if len(changes) != 1 {
		t.Errorf("invalid number of changes after file re-add: %#v", changes)
		return
	}
	if !(changes[0].Next != nil) {
		t.Errorf("should've detected file adding %#v", changes[0])
	}
}

type changesByPath []*Change

func (a changesByPath) Len() int      { return len(a) }
func (a changesByPath) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a changesByPath) Less(i, j int) bool {
	aname := ""
	if a[i].Next != nil {
		aname = a[i].Next.Path
	} else if a[i].Prev != nil {
		aname = a[i].Prev.Path
	}

	bname := ""
	if a[j].Next != nil {
		bname = a[j].Next.Path
	} else if a[j].Prev != nil {
		bname = a[j].Prev.Path
	}
	return aname < bname
}

func names(sources []*Source) []string {
	r := []string{}
	for _, src := range sources {
		r = append(r, src.Path)
	}
	return r
}

func sameFiles(sources []*Source, expected []string) bool {
	if len(sources) != len(expected) {
		return false
	}
	for i, src := range sources {
		if src.Path != expected[i] {
			return false
		}
	}
	return true
}

type filesystem map[string]string

var pseudoTime, _ = time.Parse(time.RFC1123, time.RFC1123)

func (fs filesystem) Open(name string) (http.File, error) {
	pseudoTime = pseudoTime.Add(time.Second)
	if data, ok := fs[name]; ok {
		return &file{name, pseudoTime, bytes.NewReader([]byte(data))}, nil
	}
	return nil, os.ErrNotExist
}

type file struct {
	name string
	time time.Time
	*bytes.Reader
}

// implement http.File
func (f *file) Close() error                             { return nil }
func (f *file) Readdir(count int) ([]os.FileInfo, error) { return nil, nil }
func (f *file) Stat() (os.FileInfo, error)               { return f, nil }

// implement os.FileInfo
func (f *file) Name() string       { return filepath.Base(f.name) }
func (f *file) Size() int64        { return int64(f.Len()) }
func (f *file) Mode() os.FileMode  { return os.ModePerm }
func (f *file) ModTime() time.Time { return f.time }
func (f *file) IsDir() bool        { return false }
func (f *file) Sys() interface{}   { return nil }
