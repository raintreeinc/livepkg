package livepkg

import (
	"errors"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

var ErrUnknownImport = errors.New("unknown import format")

// Source represents a single source file
type Source struct {
	Path string   `json:"path"` // absolute path for file
	Deps []string `json:"deps"` // list of absolute paths
	Ext  string   `json:"ext"`  // file extension

	ModTime     time.Time `json:"modified"`    // last modified time
	ContentType string    `json:"contentType"` // file content-type

	Content   []byte `json:"-"` // original content on disk
	Processed []byte `json:"-"` // pre-processed content in some cases
}

var (
	// quick-and-dirty import finders for JS and CSS
	rxJSImport  = regexp.MustCompile(`depends\([\t\s]*["']([^"']+)["'][\t\s]*\)[\t\s]*;?`)
	rxCSSImport = regexp.MustCompile(`@depends[\t\s]+"([^"']+)"[\t\s]*;`)
)

// ReadFrom reads content and deps from io.Reader
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

// Tag returns html tag that can be included in html
func (src *Source) Tag() template.HTML {
	u, err := url.Parse(src.Path)
	if err != nil {
		return ""
	}
	path := u.EscapedPath()
	switch src.Ext {
	case ".js":
		return template.HTML(`<script src="` + path + `" type="text/javascript" >`)
	case ".css":
		return template.HTML(`<link href="` + path + `" rel="stylesheet">`)
	case ".html":
		return template.HTML(`<link href="` + path + `" rel="import">`)
	}
	mtype := mime.TypeByExtension(src.Ext)
	return template.HTML(`<link href="` + src.Path + `" type="` + string(mtype) + `">`)
}
