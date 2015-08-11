package livepkg

import (
	"bytes"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var supportedExt = map[string]bool{
	".js":  true,
	".css": true,
}

type File struct {
	Name string   `json:"-"`    // filename on disk
	Path string   `json:"path"` // absolute path for file
	Deps []string `json:"deps"` // list of absolute paths
	Ext  string   `json:"ext"`  // file extension

	ModTime     time.Time `json:"modified"`    // last modified time
	ContentType string    `json:"contentType"` // file content-type

	Content []byte `json:"-"`
}

type walkfn func(filename string, info os.FileInfo) error

func walkfiles(root string, includedirs []string, fn walkfn) error {
	return filepath.Walk(root, func(filename string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !supportedExt[filepath.Ext(filename)] {
			return nil
		}

		if len(includedirs) == 0 {
			return fn(filename, info)
		}

		file := "/" + filepath.ToSlash(filename)
		for _, prefix := range includedirs {
			if strings.HasPrefix(file, prefix) {
				return fn(filename, info)
			}
		}
		return nil
	})
}

func LoadFiles(root string, includes []string) ([]*File, error) {
	files := []*File{}

	err := walkfiles(root, includes, func(filename string, info os.FileInfo) error {
		file, err := LoadFile(root, filename)
		if err != nil {
			return err
		}
		files = append(files, file)

		return nil
	})

	return files, err
}

func LoadFile(dir, filename string) (*File, error) {
	ext := filepath.Ext(filename)

	file := &File{
		Name: filename,
		Ext:  ext,
		Path: filepath.ToSlash(filename),

		ContentType: mime.TypeByExtension(ext),
	}

	if file.Path != "" && file.Path[0] != '/' {
		file.Path = "/" + file.Path
	}

	return file, file.Load()
}

var (
	// quick-and-dirty import finder
	rxJSImport  = regexp.MustCompile(`depends\(\s*["']([^"']+)["']\s*\);?`)
	rxCSSImport = regexp.MustCompile(`@depends\s+"([^"']+)"\s*;`)
)

func (file *File) Reload() (*File, error) {
	clone := &File{
		Name: file.Name,
		Path: file.Path,
		Ext:  file.Ext,

		ContentType: file.ContentType,
	}
	return clone, clone.Load()
}

// Loads the file content from disk
func (file *File) Load() error {
	data, err := ioutil.ReadFile(file.Name)
	if err != nil {
		return err
	}

	if stat, err := os.Stat(file.Name); err == nil {
		file.ModTime = stat.ModTime()
	}

	return file.LoadFrom(bytes.NewReader(data))
}

// Loads the content from src
func (file *File) LoadFrom(src io.Reader) error {
	data, err := ioutil.ReadAll(src)
	if err != nil {
		return err
	}

	file.Deps = []string{}
	file.Content = data

	var imports []string

	switch file.Ext {
	case ".js":
		stmts := rxJSImport.FindAllStringSubmatch(string(file.Content), -1)
		for _, stmt := range stmts {
			imports = append(imports, stmt[1])
		}
	case ".css":
		stmts := rxCSSImport.FindAllStringSubmatch(string(file.Content), -1)
		for _, stmt := range stmts {
			imports = append(imports, stmt[1])
		}
	default:
		panic("unsupported file " + file.Ext)
	}

	for _, dep := range imports {
		// relative import
		if !strings.HasPrefix(dep, "/") {
			dir := path.Dir(file.Path)
			dep = path.Clean(path.Join(dir, dep))
		}
		file.Deps = append(file.Deps, dep)
	}

	return nil
}
