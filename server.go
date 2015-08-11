package livepkg

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"time"

	"golang.org/x/net/websocket"
)

type Server struct {
	Dir   string   // directory where to serve files
	Roots []string // root paths that the server serves

	dev     bool
	sources *Sources
	socket  http.Handler
}

func NewServer(rootdir string, dev bool) *Server {
	server := &Server{
		Dir:   rootdir,
		Roots: []string{},

		dev:     dev,
		sources: NewSources(rootdir),
	}
	server.socket = websocket.Handler(server.livechanges)
	return server
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if server.dev {
		server.live(w, r)
	} else {
		server.bundle(w, r)
	}

	log.Println(r.URL.Path)
}

func (server *Server) live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "must-revalidate, no-cache")

	switch path.Base(r.URL.Path) {
	case "~pkg.js":
		w.Header().Set("Content-Type", "application/javascript")

		w.Write([]byte(jspackage))
		w.Write([]byte(jsreloader))
	case "~pkg.json":
		server.info(w, r)
	case "~pkg.css":
		// this will be handled by reloader
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{'\n'})
	case "~live":
		server.socket.ServeHTTP(w, r)
	default:
		server.sources.ServeFile(w, r)
	}
}

func (server *Server) bundle(w http.ResponseWriter, r *http.Request) {
	// TODO figure out good cache control header
	// w.Header().Set("Cache-Control", "must-revalidate, no-cache")

	switch path.Base(r.URL.Path) {
	case "~pkg.js":
		w.Header().Set("Content-Type", "application/javascript")
		data, err := server.sources.BundleByExt(".js")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(jspackage))
		w.Write(data)
	case "~pkg.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		data, err := server.sources.BundleByExt(".css")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	case "~info":
		w.WriteHeader(http.StatusForbidden)
	case "~live":
		w.WriteHeader(http.StatusForbidden)
	default:
		server.sources.ServeFile(w, r)
	}
}

func (server *Server) info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var info struct {
		Files []*File `json:"files"`
		Live  string  `json:"live"`
	}

	var err error
	info.Files, err = server.sources.Files()
	if err != nil {
		log.Println("error getting files:", err)
	}
	info.Live = path.Join(path.Dir(r.URL.Path), "~live")

	if info.Files == nil {
		info.Files = []*File{}
	}

	data, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed create JSON: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

func (server *Server) livechanges(ws *websocket.Conn) {
	// wake up client
	err := websocket.Message.Send(ws, "")
	if err != nil {
		return
	}
	defer ws.Close()

	files, _ := server.sources.Files()
	for {
		next, changes, err := server.sources.Changes(files)
		if err != nil {
			log.Println("error getting changes:", err)
			return
		}

		for _, change := range changes {
			err := websocket.JSON.Send(ws, change)
			if err != nil {
				return
			}
		}

		files = next
		time.Sleep(500 * time.Millisecond)
	}
}
