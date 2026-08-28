// Package hub implements a WebSocket fan-out hub for leaderboard updates.
package hub

import (
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client is one connected browser.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
	id   string
}

// Hub fans out messages to all connected clients.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
	maxClients int
	last       []byte // last broadcast, sent to new clients on connect
	logger     *slog.Logger
}

// NewHub constructs the hub.
func NewHub(maxClients int, logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan []byte, 256),
		maxClients: maxClients,
		logger:     logger,
	}
}

// Run is the hub event loop.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if len(h.clients) >= h.maxClients {
				h.mu.Unlock()
				close(c.send)
				continue
			}
			h.clients[c] = true
			last := h.last
			h.mu.Unlock()
			if last != nil {
				select {
				case c.send <- last:
				default:
				}
			}
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.Lock()
			h.last = msg
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Slow client: drop it rather than block the broadcast.
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Broadcast queues a message for all clients.
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcast <- msg:
	default:
	}
}

// Count returns the connected client count.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
)

func (c *Client) readPump() {
	defer func() { c.hub.unregister <- c; _ = c.conn.Close() }()
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() { ticker.Stop(); _ = c.conn.Close() }()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// newClient registers and starts pumps for a connection.
func newClient(h *Hub, conn *websocket.Conn) {
	c := &Client{hub: h, conn: conn, send: make(chan []byte, 256), id: uuid.NewString()}
	h.register <- c
	go c.writePump()
	go c.readPump()
}
