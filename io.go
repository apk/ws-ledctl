package main

import (
	"io/ioutil"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"

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

func onezero(x int) int {
	if (x == 0) {
		return 1;
	}
	return 0;
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
					op = func (x int, v int) int {
						return x | v;
					}
				case '-':
					op = func (x int, v int) int {
						return x & ^v;
					}
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
			od = d;
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

func LampHandlerStart() *hub {
	h := &hub{
		broadcast:   make(chan []byte),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
	go h.run()
	return h
}

func (h *hub) Add(mux *http.ServeMux, path string) {
	mux.Handle(path+"/ws", wsHandler{h: h})
	mux.HandleFunc(path+"/set", func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err == nil {
			h.broadcast <- body
		}
	})
}

func main() {
	mux, hid := CommonSetup()

	h := LampHandlerStart()
	if hid != nil {
		h.Add(hid,"/lamp")
	}
	h.Add(mux,"/lamp")

	CommonMain(mux, hid)
}

// g build io.go common.go
// sudo setcap cap_net_bind_service=+ep ../io
// ../io --addr :80 --path plain --tls :443 --hidpath hidden --cert-prefix certs/
