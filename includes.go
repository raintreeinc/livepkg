package livepkg

import (
	"html/template"
	"mime"
)

type Include struct {
	Ext  string `json:"ext"`  // file extension
	Path string `json:"path"` // url path
}

func (inc *Include) Tag() template.HTML {
	//TODO: verify path sanitization
	switch inc.Ext {
	case ".js":
		return template.HTML(`<script src="` + inc.Path + `" type="text/javascript" >`)
	case ".css":
		return template.HTML(`<link href="` + inc.Path + `" rel="stylesheet">`)
		// case HTML:
		//    return `<link href="` + inc.Path + `" rel="import">`
	}
	mtype := mime.TypeByExtension(inc.Ext)
	return template.HTML(`<link href="` + inc.Path + `" type="` + string(mtype) + `">`)
}

func (server *Server) Includes() []Include {
	files, _ := server.sources.Files()
	incl := []Include{}

	for _, file := range files {
		incl = append(incl, Include{
			Ext:  file.Ext,
			Path: file.Path,
		})
	}

	return incl
}
