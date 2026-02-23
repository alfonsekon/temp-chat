package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

var addr = flag.String("addr", ":8080", "http service address")

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var userIDCounter uint64

type Client struct {
	id       uint64
	username string
	conn     *websocket.Conn
	room     *Room
}

type Room struct {
	name     string
	password string
	private  bool
	clients  map[*websocket.Conn]*Client
	mu       sync.RWMutex
}

type Hub struct {
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	message    chan *Message
	mu         sync.RWMutex
}

func (h *Hub) getUniqueUsername(username string, room *Room) string {
	room.mu.RLock()
	defer room.mu.RUnlock()

	usernameExists := func(name string) bool {
		for _, c := range room.clients {
			if c.username == name {
				return true
			}
		}
		return false
	}

	if !usernameExists(username) {
		return username
	}

	rand.Seed(time.Now().UnixNano())
	for i := 1; i <= 100; i++ {
		newName := fmt.Sprintf("%s%d", username, i)
		if !usernameExists(newName) {
			return newName
		}
	}
	return fmt.Sprintf("%s%x", username, time.Now().UnixNano())
}

type Message struct {
	room      *Room
	senderID  uint64
	senderMsg []byte
	sysMsg    []byte
}

func newHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		message:    make(chan *Message),
	}
}

func (h *Hub) createRoom(name, password string, isPrivate bool) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[name]; ok {
		return nil, false
	}

	var hashedPassword string
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Failed to hash password: %v", err)
			return nil, false
		}
		hashedPassword = string(hash)
	}

	room := &Room{
		name:     name,
		password: hashedPassword,
		private:  isPrivate,
		clients:  make(map[*websocket.Conn]*Client),
	}
	h.rooms[name] = room
	return room, true
}

func (h *Hub) getRoom(name string) *Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, ok := h.rooms[name]; ok {
		return room
	}
	return nil
}

func (h *Hub) checkRoomPassword(name, password string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if room, ok := h.rooms[name]; ok {
		if room.password == "" {
			return true
		}
		err := bcrypt.CompareHashAndPassword([]byte(room.password), []byte(password))
		return err == nil
	}
	return false
}

func (h *Hub) removeRoom(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[name]; ok {
		room.mu.Lock()
		if len(room.clients) == 0 {
			delete(h.rooms, name)
		}
		room.mu.Unlock()
	}
}

func (h *Hub) broadcastToRoom(room *Room, senderID uint64, data []byte) {
	room.mu.RLock()
	for _, client := range room.clients {
		err := client.conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			client.conn.Close()
			delete(room.clients, client.conn)
		}
	}
	room.mu.RUnlock()
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			room := client.room
			room.mu.Lock()
			room.clients[client.conn] = client
			roomCount := len(room.clients)
			room.mu.Unlock()
			displayName := client.username
			if displayName == "" {
				displayName = fmt.Sprintf("User %d", client.id)
			}
			h.broadcastToRoom(room, 0, []byte(fmt.Sprintf("SYS: %s joined. Users in room: %d", displayName, roomCount)))

		case client := <-h.unregister:
			room := client.room
			room.mu.Lock()
			if _, ok := room.clients[client.conn]; ok {
				delete(room.clients, client.conn)
				client.conn.Close()
				roomCount := len(room.clients)
				room.mu.Unlock()
				displayName := client.username
				if displayName == "" {
					displayName = fmt.Sprintf("User %d", client.id)
				}
				h.broadcastToRoom(room, 0, []byte(fmt.Sprintf("SYS: %s left. Users in room: %d", displayName, roomCount)))
				if roomCount == 0 {
					h.removeRoom(room.name)
				}
			} else {
				room.mu.Unlock()
			}

		case msg := <-h.message:
			h.broadcastToRoom(msg.room, msg.senderID, msg.senderMsg)
		}
	}
}

var hub = newHub()

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomName := r.URL.Query().Get("room")
	username := r.URL.Query().Get("username")
	action := r.URL.Query().Get("action")
	roomPassword := r.URL.Query().Get("password")

	if roomName == "" {
		roomName = "default"
	}
	if username == "" {
		username = fmt.Sprintf("Guest%d", atomic.AddUint64(&userIDCounter, 1))
	}

	isPrivate := r.URL.Query().Get("private") == "true"

	var room *Room
	if action == "create" {
		createdRoom, ok := hub.createRoom(roomName, roomPassword, isPrivate)
		if !ok {
			http.Error(w, "Room already exists", http.StatusConflict)
			return
		}
		room = createdRoom
	} else {
		room = hub.getRoom(roomName)
		if room == nil {
			room, _ = hub.createRoom(roomName, "", false)
		} else if !hub.checkRoomPassword(roomName, roomPassword) {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	uniqueUsername := hub.getUniqueUsername(username, room)
	client := &Client{id: atomic.AddUint64(&userIDCounter, 1), username: uniqueUsername, conn: conn, room: room}

	hub.register <- client

	go func() {
		defer func() {
			hub.unregister <- client
		}()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			displayName := username
			if displayName == "" {
				displayName = fmt.Sprintf("User %d", client.id)
			}
			hub.message <- &Message{room: room, senderID: client.id, senderMsg: []byte(fmt.Sprintf("[%s] %s", displayName, string(message)))}
		}
	}()
}

type RoomInfo struct {
	Name      string `json:"name"`
	HasPass   bool   `json:"hasPass"`
	UserCount int    `json:"userCount"`
}

func handleRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" || token != "public-chat-token" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	rooms := make([]RoomInfo, 0, len(hub.rooms))
	for _, room := range hub.rooms {
		room.mu.RLock()
		if room.private {
			room.mu.RUnlock()
			continue
		}
		info := RoomInfo{
			Name:      room.name,
			HasPass:   room.password != "",
			UserCount: len(room.clients),
		}
		rooms = append(rooms, info)
		room.mu.RUnlock()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]RoomInfo{"rooms": rooms})
}

func main() {
	flag.Parse()
	go hub.run()

	fs := http.FileServer(http.Dir("./build"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/rooms", handleRooms)

	log.Printf("Server starting on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
