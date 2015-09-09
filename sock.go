// GOPATH=`pwd` go build sock.go && echo ok && ./sock

package main

import (
	"flag"
	"os/exec"
	"io/ioutil"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"text/template"

	"iow"
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

func (h *hub) run() {
	var r, g int;
	d := 0xf000
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
			if len(m) > 0 {
				switch m[0] {
				case 'r':
					r ^= 1
					ioutil.WriteFile("/sys/class/gpio/gpio22/value",[]byte(fmt.Sprintf("%d", r)), 0644)
				case 'g':
					g ^= 1
					ioutil.WriteFile("/sys/class/gpio/gpio17/value",[]byte(fmt.Sprintf("%d", g)), 0644)
				case 'd':
					d ^= 0x4000
				case 't':
					d ^= 0x1000
				case 'l':
					d ^= 0x2000
				case 'e':
					d ^= 0x8000
				}
			}
			m = []byte(fmt.Sprintf("%dr%dg%dd%dt%dl%de", r, g,
				onezero(d & 0x4000),
				onezero(d & 0x1000),
				onezero(d & 0x2000),
				onezero(d & 0x8000)))
			fmt.Printf("M: %s\n", m)
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
			iow.Set(w, d)
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

var (
	addr      = flag.String("addr", ":8080", "http service address")
	pic       = flag.Bool("pic", false, "pic-only mode")
	homeTempl *template.Template
)

func homeHandler(c http.ResponseWriter, req *http.Request) {
	homeTempl.Execute(c, req.Host)
}

type picreq struct {
	ch chan []byte
	size int
}

func to_s(x int) string {
	return fmt.Sprintf("%d", x)
}

func picserve(ch chan picreq) {
	for rq := range ch {
		cmd := exec.Command(
			"raspistill",
			"-t", "5",
			"-w", to_s (9 * 4 * rq.size),
			"-h", to_s (9 * 3 * rq.size),
			"-mm", "matrix",
			"-o", "-")
		out, err := cmd.Output()
		if err != nil {
			log.Print("Exec:", err)
		}
		rq.ch <- out
	}
}

func main() {
	flag.Parse()

	if (!*pic) {
		homeTempl = template.Must(template.ParseFiles("home.html"))
		http.HandleFunc("/go/", homeHandler)

		h := newHub()
		go h.run()

		http.Handle("/go/ws", wsHandler{h: h})

		http.HandleFunc("/go/set", func(w http.ResponseWriter, r *http.Request) {
			body, err := ioutil.ReadAll(r.Body)
			if err == nil {
				h.broadcast <- body
			}
		})
	}

	ch := make(chan picreq)

	go picserve(ch)

	http.HandleFunc("/go/pic", func(w http.ResponseWriter, r *http.Request) {
		_, err := ioutil.ReadAll(r.Body)
		if err == nil {
			rc := make(chan []byte)
			ch <- picreq{ch: rc, size: 8}
			s := <-rc
			w.Write([]byte(s))
		}
	})

	defhdlr := func (suf string, fac int) {
		http.HandleFunc("/go/pic" + suf, func(w http.ResponseWriter, r *http.Request) {
			_, err := ioutil.ReadAll(r.Body)
			if err == nil {
				rc := make(chan []byte)
				ch <- picreq{ch: rc, size: fac}
				s := <-rc
				w.Write([]byte(s))
			}
		})
	}

	defhdlr ("/r", 4 * 9);
	defhdlr ("/s", 2 * 9);
	defhdlr ("/t", 15);
	defhdlr ("/u", 9);
	defhdlr ("/v", 5);

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
