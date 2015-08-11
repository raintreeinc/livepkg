package main

import (
	"flag"
	"net/http"

	"github.com/raintreeinc/livepkg"
)

var (
	addr = flag.String("listen", ":8000", "address to listen on")
)

func main() {
	flag.Parse()

	dir := ""
	if flag.Arg(0) != "" {
		dir = flag.Arg(0)
	}

	server := livepkg.NewServer(dir)
	http.ListenAndServe(*addr, server)
}
