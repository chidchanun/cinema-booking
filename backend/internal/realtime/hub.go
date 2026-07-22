package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// roomMessage เป็นข้อมูลภายใน Hub โดยระบุ room ปลายทางและ JSON payload
type roomMessage struct {
	showtimeID string
	payload    []byte
}

// Hub เป็นศูนย์กลางจัดการ WebSocket clients แยกตาม showtime room
// ทุกการแก้ rooms map ทำผ่าน Run event loop เพียง goroutine เดียว
type Hub struct {
	rooms map[string]map[*Client]struct{}

	register   chan *Client
	unregister chan *Client
	broadcast  chan roomMessage

	done     chan struct{}
	doneOnce sync.Once
}

// NewHub สร้าง channels และ rooms map โดย broadcastBuffer กำหนดจำนวน
// event ที่รอส่งได้ก่อน Publish คืน ErrRealtimeQueueFull
func NewHub(
	broadcastBuffer int,
) *Hub {
	if broadcastBuffer <= 0 {
		broadcastBuffer = 256
	}

	return &Hub{
		rooms: make(
			map[string]map[*Client]struct{},
		),

		register: make(
			chan *Client,
			64,
		),
		unregister: make(
			chan *Client,
			64,
		),
		broadcast: make(
			chan roomMessage,
			broadcastBuffer,
		),

		done: make(chan struct{}),
	}
}

// Run ประมวลผล register, unregister และ broadcast ตามลำดับที่ channel ส่งมา
// ต้องเรียกหนึ่งครั้งใน goroutine และส่ง context ที่ถูกยกเลิกตอนปิด application
func (h *Hub) Run(
	ctx context.Context,
) {
	// Event loop นี้เป็นเจ้าของ rooms map เพียง goroutine เดียว การ register,
	// unregister และ broadcast จึงไม่ต้องใช้ mutex และไม่เกิด concurrent map access
	defer h.doneOnce.Do(func() {
		close(h.done)
	})

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastToRoom(message)

		case <-ctx.Done():
			h.closeAllClients()
			return
		}
	}
}

// Register ส่ง client เข้า event loop โดยรอจนส่งสำเร็จ, Hub ปิด หรือ context หมดเวลา
func (h *Hub) Register(
	ctx context.Context,
	client *Client,
) error {
	if client == nil {
		return fmt.Errorf("realtime client is nil")
	}

	select {
	case h.register <- client:
		return nil

	case <-h.done:
		return ErrRealtimeClosed

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Unregister ขอให้ event loop ลบ client และปิด send channel
// เป็น non-blocking เพื่อไม่ให้ cleanup ของ connection ค้างเมื่อ Hub กำลังปิด
func (h *Hub) Unregister(
	client *Client,
) {
	if client == nil {
		return
	}

	select {
	case h.unregister <- client:
	case <-h.done:
	default:
	}
}

// Publish แปลง SeatEvent เป็น JSON แล้วใส่ broadcast queue ของ room ที่เกี่ยวข้อง
// ฟังก์ชันไม่รอให้ client ทุกคนเขียนข้อมูลเสร็จ จึงตอบกลับ business flow ได้เร็ว
func (h *Hub) Publish(
	ctx context.Context,
	event SeatEvent,
) error {
	event.ShowtimeID = strings.TrimSpace(
		event.ShowtimeID,
	)
	event.SeatCode = strings.ToUpper(
		strings.TrimSpace(event.SeatCode),
	)

	if event.ShowtimeID == "" {
		return fmt.Errorf(
			"realtime event showtime ID is required",
		)
	}

	// Marshal แปลง struct เป็น JSON text frame ที่ frontend อ่านได้
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf(
			"marshal realtime event: %w",
			err,
		)
	}

	message := roomMessage{
		showtimeID: event.ShowtimeID,
		payload:    payload,
	}

	// default ทำให้ queue เต็มแล้ว fail ทันที แทนการขวาง booking/seat-lock request
	select {
	case h.broadcast <- message:
		return nil

	case <-h.done:
		return ErrRealtimeClosed

	case <-ctx.Done():
		return ctx.Err()

	default:
		return ErrRealtimeQueueFull
	}
}

// registerClient เพิ่ม client เข้า room และสร้าง room เมื่อมีสมาชิกคนแรก
// ต้องเรียกจาก Run event loop เท่านั้น
func (h *Hub) registerClient(
	client *Client,
) {
	showtimeID := client.showtimeID

	if h.rooms[showtimeID] == nil {
		h.rooms[showtimeID] = make(
			map[*Client]struct{},
		)
	}

	h.rooms[showtimeID][client] = struct{}{}
}

// unregisterClient ลบ client, ปิด outbound channel และลบ room ที่ไม่มีสมาชิก
// ต้องเรียกจาก Run event loop เท่านั้นเพื่อรักษา ownership ของ rooms map
func (h *Hub) unregisterClient(
	client *Client,
) {
	clients, exists := h.rooms[client.showtimeID]
	if !exists {
		return
	}

	if _, exists := clients[client]; !exists {
		return
	}

	delete(clients, client)
	close(client.send)

	if len(clients) == 0 {
		delete(h.rooms, client.showtimeID)
	}
}

// broadcastToRoom กระจาย payload ไปยัง send channel ของ client ทุกคนใน showtime
// client ที่ตามข้อมูลไม่ทันจะถูกตัดออกเพื่อรักษาความพร้อมของ room โดยรวม
func (h *Hub) broadcastToRoom(
	message roomMessage,
) {
	clients, exists := h.rooms[message.showtimeID]
	if !exists {
		return
	}

	for client := range clients {
		select {
		case client.send <- message.payload:

		default:
			// ตัด client ที่อ่านช้าจน send buffer เต็ม เพื่อไม่ให้ client เดียว
			// ขวางการ broadcast สถานะที่นั่งไปยังผู้ใช้อื่นทั้งห้อง
			delete(clients, client)
			close(client.send)
			_ = client.connection.Close()
		}
	}

	if len(clients) == 0 {
		delete(h.rooms, message.showtimeID)
	}
}

// closeAllClients ปิด outbound channel และ network connection ทุกตัวตอน shutdown
func (h *Hub) closeAllClients() {
	for showtimeID, clients := range h.rooms {
		for client := range clients {
			close(client.send)
			_ = client.connection.Close()
		}

		delete(h.rooms, showtimeID)
	}
}
