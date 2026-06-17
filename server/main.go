package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serveWs(hub *Hub, game *Game, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan ServerMessage, 256),
		game: game,
	}

	var initialMsg ClientMessage
	err = conn.ReadJSON(&initialMsg)
	if err != nil {
		log.Println("initial read error:", err)
		conn.Close()
		return
	}

	client.hub.register <- client

	name := "Player"
	if initialMsg.Type == MsgHello {
		dataBytes, _ := json.Marshal(initialMsg.Data)
		var hello HelloMessage
		if err := json.Unmarshal(dataBytes, &hello); err == nil && hello.PlayerName != "" {
			name = hello.PlayerName
		}
	}

	game.SendWelcome(client, name)

	go client.WritePump()
	go client.ReadPump()
}

func Main() {
	port := 8080
	if len(os.Args) > 1 {
		if p, err := strconv.Atoi(os.Args[1]); err == nil {
			port = p
		}
	}

	seed := time.Now().UnixNano()
	if len(os.Args) > 2 {
		if s, err := strconv.ParseInt(os.Args[2], 10, 64); err == nil {
			seed = s
		}
	}

	game := NewGame(seed)
	game.SpawnMonsters()

	hub := NewHub(game)
	game.SetHub(hub)
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, game, w, r)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Roguelike Server running. Connect via WebSocket to /ws")
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Server starting on %s, seed=%d", addr, seed)
	log.Fatal(http.ListenAndServe(addr, nil))
}
