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
var ssladdr = flag.String("tls", "", "http service address")
var certpref = flag.String("cert-prefix", "", "prefix for cert files")
var docroot = flag.String("path", ".", "http root directory")
var iphead = flag.String("ip", "", "header for remote IP")
var wellknown = flag.String("well-known", "banana.h.apk.li", "host for .well-known")

func CommonSetup() *http.ServeMux {
	mux := http.NewServeMux()
	flag.Parse()
	pth, err := filepath.Abs(*docroot)
	if (err != nil) {
		fmt.Printf("filepath.Abs(%v): %v\n",*docroot,err)
		return nil
	}

	mux.HandleFunc("/", gnord.GnordHandleFunc(&gnord.GnordOpts{Path: pth, IpHeader: *iphead}))

	mux.HandleFunc("/.well-known/", gnord.SSLForwarderHandleFunc(*wellknown))

	gnord.PiCam(mux,"/pic")
	return mux;
}

func CommonMain(mux *http.ServeMux) {
	if *ssladdr != "" {
		go func () {
			log.Fatal(http.ListenAndServeTLS(*ssladdr,
				*certpref + "fullchain.pem", *certpref + "key.pem",
				mux))
		} ()
	}
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// sudo setcap cap_net_bind_service=+ep ./pic
