package main

import (
	"flag"
	"net/http"

	"github.com/raintreeinc/livepkg"
)

var (
	addr = flag.String("listen", ":8000", "address to listen on")
	dev  = flag.Bool("dev", true, "development mode")
	root = flag.String("root", ".", "root directory")
)

func main() {
	flag.Parse()
	pkg := livepkg.NewServer(http.Dir(*root), *dev, flag.Args()...)
	http.ListenAndServe(*addr, pkg)
}
