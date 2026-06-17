package server

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients    map[int]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan ServerMessage
	mu         sync.RWMutex
	nextID     int
}

type Client struct {
	ID       int
	conn     *websocket.Conn
	hub      *Hub
	send     chan ServerMessage
	player   *Player
	mu       sync.Mutex
	game     *Game
}

func NewHub(game *Game) *Hub {
	return &Hub{
		clients:    make(map[int]*Client),
		register:   make(chan *Client, 256),
		unregister: make(chan *Client, 256),
		broadcast:  make(chan ServerMessage, 256),
		nextID:     1,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			client.ID = h.nextID
			h.nextID++
			h.clients[client.ID] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.send <- msg:
				default:
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		var msg ClientMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			return
		}

		c.game.HandleClientMessage(c, msg)
	}
}

func (c *Client) WritePump() {
	defer c.conn.Close()

	for msg := range c.send {
		err := c.conn.WriteJSON(msg)
		if err != nil {
			return
		}
	}
}

func (c *Client) Send(msg ServerMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case c.send <- msg:
	default:
	}
}
