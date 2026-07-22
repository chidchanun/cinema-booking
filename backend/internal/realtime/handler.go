package realtime

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cinema-booking/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ShowtimeReader ใช้ตรวจว่ารอบฉายมีอยู่และเปิดให้ผู้ใช้เข้าดูได้ก่อนสร้าง WebSocket
// การพึ่ง interface ทำให้ Handler ไม่ผูกกับ implementation ของ service โดยตรง
type ShowtimeReader interface {
	GetPublicShowtime(
		ctx context.Context,
		id primitive.ObjectID,
	) (*models.Showtime, error)
}

// Handler รับ HTTP request และเปลี่ยน protocol จาก HTTP เป็น WebSocket
// จากนั้นสร้าง Client และนำเข้า room ของรอบฉายผ่าน Hub
type Handler struct {
	hub            *Hub
	showtimeReader ShowtimeReader
	upgrader       websocket.Upgrader
}

// NewHandler สร้าง WebSocket handler พร้อม origin allowlist และ buffer ของ connection
// allowedOrigins ต้องตรงกับ origin ของ frontend เช่น http://localhost:5173
func NewHandler(
	hub *Hub,
	showtimeReader ShowtimeReader,
	allowedOrigins []string,
) (*Handler, error) {
	if hub == nil {
		return nil, fmt.Errorf(
			"realtime handler: hub is nil",
		)
	}

	if showtimeReader == nil {
		return nil, fmt.Errorf(
			"realtime handler: showtime reader is nil",
		)
	}

	originSet := make(
		map[string]struct{},
		len(allowedOrigins),
	)

	for _, rawOrigin := range allowedOrigins {
		origin := strings.TrimSpace(rawOrigin)

		if origin == "" {
			continue
		}

		originSet[origin] = struct{}{}
	}

	if len(originSet) == 0 {
		return nil, fmt.Errorf(
			"realtime handler: allowed origins cannot be empty",
		)
	}

	handler := &Handler{
		hub:            hub,
		showtimeReader: showtimeReader,
	}

	// Upgrader กำหนดวิธีเปลี่ยน HTTP connection เป็น WebSocket connection
	handler.upgrader = websocket.Upgrader{
		// Buffer ใช้เก็บข้อมูลระหว่างอ่านและเขียน frame เพื่อลด allocation
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,

		// เปิด per-message compression สำหรับ payload ที่ client รองรับ
		EnableCompression: true,

		CheckOrigin: func(
			request *http.Request,
		) bool {
			// WebSocket ไม่ใช้ CORS preflight จึงต้องตรวจ Origin ที่ขั้นตอน
			// handshake เพื่อป้องกันเว็บไซต์ที่ไม่ได้รับอนุญาตเปิด connection
			origin := strings.TrimSpace(
				request.Header.Get("Origin"),
			)

			/*
				Browser จะส่ง Origin มาเสมอ

				อนุญาตค่าว่างสำหรับ CLI หรือ Testing Tool
				ที่ไม่มี Origin header
			*/
			if origin == "" {
				return true
			}

			_, allowed := originSet[origin]

			return allowed
		},
	}

	return handler, nil
}

// Connect ตรวจ path parameter และรอบฉายก่อนทำ WebSocket handshake
// เมื่อสำเร็จจะ register client ใน room ที่ใช้ showtime ID เป็น key
func (h *Handler) Connect(
	c *gin.Context,
) {
	showtimeID, err := primitive.ObjectIDFromHex(
		strings.TrimSpace(
			c.Param("showtimeID"),
		),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_showtime_id",
			"message": "Showtime ID is invalid",
		})
		return
	}

	/*
		ตรวจสอบก่อน Upgrade

		หลัง Upgrade แล้วไม่ควรตอบ HTTP JSON Error
	*/
	if _, err := h.showtimeReader.GetPublicShowtime(
		c.Request.Context(),
		showtimeID,
	); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "showtime_not_found",
			"message": "Showtime not found or unavailable",
		})
		return
	}

	// Upgrade ส่ง HTTP 101 Switching Protocols และคืน WebSocket connection
	// หลังคำสั่งนี้ห้ามตอบ c.JSON เพราะ connection ไม่ใช่ HTTP response แล้ว
	connection, err := h.upgrader.Upgrade(
		c.Writer,
		c.Request,
		nil,
	)
	if err != nil {
		return
	}

	client := NewClient(
		h.hub,
		connection,
		showtimeID.Hex(),
	)

	// จำกัดเวลารอส่ง client เข้า register channel ป้องกัน request ค้าง
	// หาก Hub หยุดทำงานหรือมีงานสะสมมากเกินไป
	registerCtx, registerCancel :=
		context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
	defer registerCancel()

	if err := h.hub.Register(
		registerCtx,
		client,
	); err != nil {
		_ = connection.Close()
		return
	}

	// แต่ละ connection มี reader และ writer อย่างละหนึ่ง goroutine ตามข้อกำหนด
	// ของ Gorilla WebSocket เพื่อไม่ให้เกิด concurrent read/write บน connection
	go client.WritePump()
	go client.ReadPump()
}
