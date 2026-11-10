package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = 54 * time.Second
	maxMessageSize  = 8192
	sendBufferSize  = 256
	enqueueDropped  = "realtime: dropped message, client send buffer full"
)

type Message struct {
	Type      string `json:"type"`
	ProjectID uint   `json:"project_id,omitempty"`
	Data      any    `json:"data,omitempty"`
}

type MemberChecker func(projectID, userID uint) (bool, error)

// Hub keeps track of connected WebSocket clients and fans messages out to
// per-user rooms (notifications) and per-project rooms (collaboration events).
type Hub struct {
	mu             sync.RWMutex
	clients        map[*Client]struct{}
	projectRooms   map[uint]map[*Client]bool
	userRooms      map[uint]map[*Client]bool
	isMember       MemberChecker
	allowedOrigins []string
}

func NewHub(allowedOrigins []string, isMember MemberChecker) *Hub {
	return &Hub{
		clients:        make(map[*Client]struct{}),
		projectRooms:   make(map[uint]map[*Client]bool),
		userRooms:      make(map[uint]map[*Client]bool),
		isMember:       isMember,
		allowedOrigins: allowedOrigins,
	}
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   uint
	projects map[uint]bool
}

func (c *Client) enqueue(data []byte) {
	select {
	case c.send <- data:
	default:
		log.Println(enqueueDropped)
	}
}

func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, o := range h.allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	if h.userRooms[c.userID] == nil {
		h.userRooms[c.userID] = make(map[*Client]bool)
	}
	h.userRooms[c.userID][c] = true
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	delete(h.clients, c)
	if room := h.userRooms[c.userID]; room != nil {
		delete(room, c)
		if len(room) == 0 {
			delete(h.userRooms, c.userID)
		}
	}
	for pid := range c.projects {
		if room := h.projectRooms[pid]; room != nil {
			delete(room, c)
			if len(room) == 0 {
				delete(h.projectRooms, pid)
			}
		}
	}
	close(c.send)
}

// subscribe must only be called from the client's own readPump goroutine.
func (h *Hub) subscribe(c *Client, projectID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.projectRooms[projectID] == nil {
		h.projectRooms[projectID] = make(map[*Client]bool)
	}
	h.projectRooms[projectID][c] = true
	c.projects[projectID] = true
}

func (h *Hub) unsubscribe(c *Client, projectID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room := h.projectRooms[projectID]; room != nil {
		delete(room, c)
		if len(room) == 0 {
			delete(h.projectRooms, projectID)
		}
	}
	delete(c.projects, projectID)
}

// RemoveUserFromProject forcibly drops a user's connections from a project
// room (used when a member is removed).
func (h *Hub) RemoveUserFromProject(projectID, userID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.projectRooms[projectID]
	for c := range room {
		if c.userID == userID {
			delete(room, c)
			delete(c.projects, projectID)
		}
	}
	if len(room) == 0 {
		delete(h.projectRooms, projectID)
	}
}

// CloseProject drops the whole project room (used when a project is deleted).
func (h *Hub) CloseProject(projectID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.projectRooms[projectID] {
		delete(c.projects, projectID)
	}
	delete(h.projectRooms, projectID)
}

func (h *Hub) BroadcastToProject(projectID uint, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("realtime: marshal broadcast: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.projectRooms[projectID] {
		c.enqueue(data)
	}
}

func (h *Hub) SendToUser(userID uint, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("realtime: marshal user message: %v", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.userRooms[userID] {
		c.enqueue(data)
	}
}

// ServeWS upgrades the HTTP connection and starts the client pumps.
func ServeWS(h *Hub, w http.ResponseWriter, r *http.Request, userID uint) error {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		userID:   userID,
		projects: make(map[uint]bool),
	}
	h.register(client)
	go client.writePump()
	go client.readPump()
	return nil
}

type inboundMessage struct {
	Type      string `json:"type"`
	ProjectID uint   `json:"project_id"`
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg inboundMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "subscribe":
			ok, err := c.hub.isMember(msg.ProjectID, c.userID)
			if err != nil || !ok || msg.ProjectID == 0 {
				c.reply(Message{Type: "error", ProjectID: msg.ProjectID, Data: map[string]string{"message": "forbidden"}})
				continue
			}
			c.hub.subscribe(c, msg.ProjectID)
			c.reply(Message{Type: "subscribed", ProjectID: msg.ProjectID})
		case "unsubscribe":
			c.hub.unsubscribe(c, msg.ProjectID)
			c.reply(Message{Type: "unsubscribed", ProjectID: msg.ProjectID})
		case "ping":
			c.reply(Message{Type: "pong"})
		}
	}
}

// reply routes through the send channel so all websocket writes happen on
// the single writePump goroutine (gorilla forbids concurrent writers).
func (c *Client) reply(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	c.enqueue(data)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
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
