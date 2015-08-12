package livepkg

import (
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"path"
	"regexp"
	"strings"
	"time"
)

var ErrUnknownImport = errors.New("unknown import format")

type Source struct {
	Path string   `json:"path"` // absolute path for file
	Deps []string `json:"deps"` // list of absolute paths
	Ext  string   `json:"ext"`  // file extension

	ModTime     time.Time `json:"modified"`    // last modified time
	ContentType string    `json:"contentType"` // file content-type

	Content   []byte `json:"-"`
	Processed []byte `json:"-"`
}

var (
	// quick-and-dirty import finder
	rxJSImport  = regexp.MustCompile(`depends\(\s*["']([^"']+)["']\s*\);?`)
	rxCSSImport = regexp.MustCompile(`@depends\s+"([^"']+)"\s*;`)
)

func (source *Source) ReadFrom(r io.Reader) error {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	source.Deps = []string{}
	source.Content = data
	source.Processed = data

	var imports []string

	switch source.Ext {
	case ".js":
		stmts := rxJSImport.FindAllStringSubmatch(string(source.Content), -1)
		for _, stmt := range stmts {
			imports = append(imports, stmt[1])
		}
	case ".css":
		stmts := rxCSSImport.FindAllStringSubmatch(string(source.Content), -1)
		for _, stmt := range stmts {
			imports = append(imports, stmt[1])
		}
	case ".html":
		//TODO: implement
	default:
		return ErrUnknownImport
	}

	for _, dep := range imports {
		// relative import
		if !strings.HasPrefix(dep, "/") {
			dir := path.Dir(source.Path)
			dep = path.Clean(path.Join(dir, dep))
		}
		source.Deps = append(source.Deps, dep)
	}

	return nil
}

func (src *Source) Tag() template.HTML {
	//TODO: verify path sanitization
	switch src.Ext {
	case ".js":
		return template.HTML(`<script src="` + src.Path + `" type="text/javascript" >`)
	case ".css":
		return template.HTML(`<link href="` + src.Path + `" rel="stylesheet">`)
	case ".html":
		return template.HTML(`<link href="` + src.Path + `" rel="import">`)
	}
	mtype := mime.TypeByExtension(src.Ext)
	return template.HTML(`<link href="` + src.Path + `" type="` + string(mtype) + `">`)
}
