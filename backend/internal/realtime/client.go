package realtime

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait จำกัดเวลาส่ง frame ป้องกัน socket ที่ค้างจาก client ที่ไม่อ่านข้อมูล
	writeWait = 10 * time.Second
	// pongWait คือเวลาสูงสุดที่ยอมให้ connection ไม่มี pong ก่อนถือว่าหลุด
	pongWait = 60 * time.Second

	// ส่ง ping ก่อน read deadline หมดเพื่อให้ client มีเวลาตอบ pong
	pingPeriod = pongWait * 9 / 10

	// ระบบรับเฉพาะ control frame จึงจำกัด inbound message เพื่อลดความเสี่ยงใช้หน่วยความจำเกิน
	maxMessageSize = 512
)

// Client แทน WebSocket connection หนึ่งตัวและ outbound queue ของ connection นั้น
// showtimeID ระบุ room ที่ client ต้องรับ seat events
type Client struct {
	hub *Hub

	connection *websocket.Conn
	send       chan []byte

	showtimeID string
}

// NewClient ผูก connection กับ Hub และสร้าง buffered send channel
// buffer ช่วยรองรับ event burst สั้น ๆ โดยไม่ทำให้ Hub รอ network write
func NewClient(
	hub *Hub,
	connection *websocket.Conn,
	showtimeID string,
) *Client {
	return &Client{
		hub: hub,

		connection: connection,
		send:       make(chan []byte, 64),

		showtimeID: showtimeID,
	}
}

// ReadPump อ่าน incoming/control frames ต่อเนื่องและต่ออายุ read deadline เมื่อได้รับ pong
// เมื่ออ่านไม่ได้จะ unregister client และปิด connection
func (c *Client) ReadPump() {
	// ReadPump มีหน้าที่อ่าน control frames (pong/close) แม้ระบบจะไม่รับ
	// business event จาก browser และเป็นผู้ unregister client เมื่อ connection ปิด
	defer func() {
		c.hub.Unregister(c)
		_ = c.connection.Close()
	}()

	// SetReadLimit ปฏิเสธ message ที่ใหญ่เกิน maxMessageSize
	c.connection.SetReadLimit(
		maxMessageSize,
	)

	// SetReadDeadline ทำให้ ReadMessage คืน error หาก client ไม่ตอบ pong ตามเวลา
	_ = c.connection.SetReadDeadline(
		time.Now().Add(pongWait),
	)

	// SetPongHandler ถูกเรียกอัตโนมัติเมื่อ Gorilla อ่าน pong control frame
	c.connection.SetPongHandler(
		func(string) error {
			return c.connection.SetReadDeadline(
				time.Now().Add(pongWait),
			)
		},
	)

	/*
		Client ไม่ต้องส่ง Business Event เข้ามา

		แต่ต้อง Read ต่อเนื่องเพื่อให้ Gorilla WebSocket
		ประมวลผล Pong และ Close frame
	*/
	for {
		if _, _, err := c.connection.ReadMessage(); err != nil {
			return
		}
	}

}

// WritePump เขียน seat events และ ping control frames ออกทาง connection
// ต้องมี writer เพียง goroutine เดียวตาม concurrency contract ของ Gorilla WebSocket
func (c *Client) WritePump() {
	// WritePump เป็นผู้เขียน connection เพียง goroutine เดียว ทั้ง seat event
	// และ ping heartbeat จึงถูก serialize ผ่านจุดนี้
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		_ = c.connection.Close()
	}()

	for {
		select {
		case message, open := <-c.send:
			// SetWriteDeadline จำกัดเวลาของ network write ในแต่ละรอบ
			_ = c.connection.SetWriteDeadline(
				time.Now().Add(writeWait),
			)

			if !open {
				// send channel ปิดหมายถึง Hub unregister/shutdown จึงส่ง close frame
				_ = c.connection.WriteMessage(
					websocket.CloseMessage,
					[]byte{},
				)
				return
			}

			// TextMessage ส่ง JSON SeatEvent ไปยัง frontend
			if err := c.connection.WriteMessage(
				websocket.TextMessage,
				message,
			); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.connection.SetWriteDeadline(
				time.Now().Add(writeWait),
			)

			// PingMessage ตรวจว่า peer ยังเชื่อมต่ออยู่และกระตุ้นให้ตอบ pong
			if err := c.connection.WriteMessage(
				websocket.PingMessage,
				nil,
			); err != nil {
				return
			}
		}
	}
}
