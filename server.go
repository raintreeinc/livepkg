package livepkg

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Server implements a live reloading server for JS
type Server struct {
	root   http.FileSystem
	main   []string
	dev    bool
	socket http.Handler

	once   sync.Once
	bundle *Bundle

	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

// NewServer returns a new server
func NewServer(root http.FileSystem, dev bool, main ...string) *Server {
	server := &Server{
		root:    root,
		main:    main,
		dev:     dev,
		bundle:  NewBundle(root, main...),
		clients: make(map[*websocket.Conn]struct{}),
	}
	server.socket = websocket.Handler(server.livechanges)
	return server
}

// init initializes the bundle and starts monitoring disk for changes
func (server *Server) init() {
	_, err := server.bundle.Reload()
	if err != nil {
		log.Println(err)
	}
	if server.dev {
		go server.monitor()
	}
}

// broadcast sends a change to all connected clients
func (server *Server) broadcast(change *Change) {
	server.mu.RLock()
	defer server.mu.RUnlock()
	for ws := range server.clients {
		websocket.JSON.Send(ws, change)
	}
}

// monitor monitors for changes on disk
func (server *Server) monitor() {
	for {
		changes, err := server.bundle.Reload()
		if err != nil {
			log.Println(err)
		}
		if len(changes) > 0 {
			for _, change := range changes {
				server.broadcast(change)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ServeHTTP implements http.Server
func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.once.Do(server.init)
	if server.dev {
		server.serveLive(w, r)
	} else {
		server.serveBundle(w, r)
	}
}

// serveLive serves reloader and serves the web socket connection
func (server *Server) serveLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "must-revalidate, no-cache")

	switch path.Base(r.URL.Path) {
	case "~pkg.js":
		w.Header().Set("Content-Type", "application/javascript")

		w.Write([]byte(jspackage))

		origurl, err := url.ParseRequestURI(r.RequestURI)
		origpath := r.RequestURI
		if err != nil && origurl != nil {
			origpath = origurl.Path
		}
		rootpath := template.JSEscapeString(path.Dir(origpath))
		w.Write([]byte(strings.Replace(jsreloader, rootPathMarker, rootpath, -1)))
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
		server.bundle.ServeFile(w, r)
	}
}

// serveBundle serves the bundled js and css files
func (server *Server) serveBundle(w http.ResponseWriter, r *http.Request) {
	switch path.Base(r.URL.Path) {
	case "~pkg.js":
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(jspackage))
		w.Write(server.bundle.MergedByExt(".js"))
	case "~pkg.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(server.bundle.MergedByExt(".css"))
	case "~info":
		w.WriteHeader(http.StatusForbidden)
	case "~live":
		w.WriteHeader(http.StatusForbidden)
	default:
		http.FileServer(server.root).ServeHTTP(w, r)
	}
}

// info serves information about all the files
func (server *Server) info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var info struct {
		Files []*Source `json:"files"`
	}

	var err error
	info.Files = server.bundle.All()

	data, err := json.MarshalIndent(info, "", "\t")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed create JSON: %v", err), http.StatusInternalServerError)
		return
	}

	w.Write(data)
}

// livechanges handles live reloader connection
func (server *Server) livechanges(ws *websocket.Conn) {
	// wake up client
	err := websocket.Message.Send(ws, "")
	if err != nil {
		return
	}
	defer ws.Close()

	server.mu.Lock()
	server.clients[ws] = struct{}{}
	server.mu.Unlock()

	defer func() {
		server.mu.Lock()
		delete(server.clients, ws)
		server.mu.Unlock()
	}()

	io.Copy(ioutil.Discard, ws)
}
