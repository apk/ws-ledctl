package main

import (
	"net/http"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	gnord "github.com/apk/httptools"
)

var addr = flag.String("addr", "127.0.0.1:4040", "http service address")
var docroot = flag.String("path", ".", "http root directory")
var iphead = flag.String("ip", "", "header for remote IP")
var wellknown = flag.String("well-known", "banana.h.apk.li", "host for .well-known")

func main() {
	mux := http.NewServeMux()
	flag.Parse()
	pth, err := filepath.Abs(*docroot)
	if (err != nil) {
		fmt.Printf("filepath.Abs(%v): %v\n",*docroot,err)
		return
	}

	mux.HandleFunc("/", gnord.GnordHandleFunc(&gnord.GnordOpts{Path: pth, IpHeader: *iphead}))

	mux.HandleFunc("/.well-known/", gnord.SSLForwarderHandleFunc(*wellknown))

	gnord.PiCam(mux,"/pic")

	log.Fatal(http.ListenAndServe(*addr, mux))
}
