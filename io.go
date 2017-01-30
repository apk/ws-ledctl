package main

import (
	"flag"
	//"os/exec"
	"io/ioutil"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"

	"iow"

	// ================
	
	//"net/http"
	//"flag"
	//"fmt"
	//"log"
	"path/filepath"
	gnord "github.com/apk/httptools"
)

type hub struct {
	// Registered connections.
	connections map[*connection]bool

	// Inbound messages from the connections.
	broadcast chan []byte

	// Register requests from the connections.
	register chan *connection

	// Unregister requests from connections.
	unregister chan *connection
}

func newHub() *hub {
	return &hub{
		broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
}

func onezero(x int) int {
	if (x == 0) {
		return 0;
	}
	return 1;
}

func op_set(x int, v int) int {
	return x | v;
}

func op_clr(x int, v int) int {
	return x & ^v;
}

func op_xor(x int, v int) int {
	return x ^ v;
}

func (h *hub) run() {
	var r, g int;
	d := 0x0000
	od := 0
	w := iow.Open()
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		case m := <-h.broadcast:
			fmt.Printf("M: %s\n", m)
			op := op_xor
			for _,mm := range m {
				switch mm {
				case '+':
					op = op_set;
				case '-':
					op = op_clr;
				case '/':
					op = op_xor;
				case 'r':
					r = op(r, 1)
					ioutil.WriteFile("/sys/class/gpio/gpio22/value",[]byte(fmt.Sprintf("%d", r)), 0644)
				case 'g':
					g = op(g, 1)
					ioutil.WriteFile("/sys/class/gpio/gpio17/value",[]byte(fmt.Sprintf("%d", g)), 0644)
				case 'd':
					d = op(d, 0x4000)
				case 't':
					d = op(d, 0x1000)
				case 'l':
					d = op(d, 0x2000)
				case 'e':
					d = op(d, 0x8000)
				}
			}
			m = []byte(fmt.Sprintf("%dr%dg%dd%dt%dl%de", r, g,
				onezero(d & 0x4000),
				onezero(d & 0x1000),
				onezero(d & 0x2000),
				onezero(d & 0x8000)))
			fmt.Printf("R: %s\n", m)
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					close(c.send)
				}
			}
		}
		if (d != od) {
			fmt.Printf("%d %x\n", w, d);
			iow.Set(w, d ^ 0xf000)
		}
	}
}

type connection struct {
	// The websocket connection.
	ws *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// The hub.
	h *hub
}

func (c *connection) reader() {
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
		c.h.broadcast <- message
	}
	c.ws.Close()
}

func (c *connection) writer() {
	for message := range c.send {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			break
		}
	}
	c.ws.Close()
}

var upgrader = &websocket.Upgrader{
	ReadBufferSize: 1024, WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsHandler struct {
	h *hub
}

func (wsh wsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &connection{send: make(chan []byte, 256), ws: ws, h: wsh.h}
	c.h.register <- c
	defer func() { c.h.unregister <- c }()
	go c.writer()
	c.reader()
}


func LampHandler(mux *http.ServeMux, path string) {
	h := newHub()
	go h.run()
	mux.Handle(path, wsHandler{h: h})
}


var addr = flag.String("addr", "127.0.0.1:4040", "http service address")
var ssladdr = flag.String("tls", "", "http service address")
var certpref = flag.String("cert-prefix", "", "prefix for cert files")
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
