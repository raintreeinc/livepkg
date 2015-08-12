package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/raintreeinc/livepkg"
)

var (
	addr = flag.String("listen", ":8000", "http server `address`")
	dev  = flag.Bool("dev", true, "development mode")
)

func main() {
	flag.Parse()

	if os.Getenv("HOST") != "" || os.Getenv("PORT") != "" {
		*addr = os.Getenv("HOST") + ":" + os.Getenv("PORT")
	}

	dir := http.Dir(".")
	pkg := livepkg.NewServer(dir, *dev, "/ui/main.js", "/ui/main.css")
	http.Handle("/ui/", pkg)
	http.HandleFunc("/", index)

	assets := http.Dir("assets")
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(assets)))

	log.Println("starting listening on ", *addr)
	http.ListenAndServe(*addr, nil)
}

func index(w http.ResponseWriter, r *http.Request) {
	T := template.Must(template.New("").ParseFiles("index.html"))
	err := T.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		log.Println(err)
	}
}
