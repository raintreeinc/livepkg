package livepkg

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

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

func (server *Server) init() {
	_, err := server.bundle.Reload()
	if err != nil {
		log.Println(err)
	}
	if server.dev {
		go server.monitor()
	}
}

func (server *Server) broadcast(change *Change) {
	server.mu.RLock()
	defer server.mu.RUnlock()
	for ws := range server.clients {
		websocket.JSON.Send(ws, change)
	}
}

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

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.once.Do(server.init)
	if server.dev {
		server.serveLive(w, r)
	} else {
		server.serveBundle(w, r)
	}
}

func (server *Server) serveLive(w http.ResponseWriter, r *http.Request) {
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
		server.bundle.ServeFile(w, r)
	}
}

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

func (server *Server) info(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var info struct {
		Files []*Source `json:"files"`
		Live  string    `json:"live"`
	}

	var err error
	info.Files = server.bundle.All()
	info.Live = path.Join(path.Dir(r.URL.Path), "~live")

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
