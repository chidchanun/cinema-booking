package observability

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

var (
	httpRequests        atomic.Uint64
	httpClientErrors    atomic.Uint64
	httpServerErrors    atomic.Uint64
	webSocketUpgrades   atomic.Uint64
	webSocketRejections atomic.Uint64
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		httpRequests.Add(1)
		status := c.Writer.Status()
		if status >= http.StatusInternalServerError {
			httpServerErrors.Add(1)
		} else if status >= http.StatusBadRequest {
			httpClientErrors.Add(1)
		}
	}
}

func RecordWebSocketUpgrade()   { webSocketUpgrades.Add(1) }
func RecordWebSocketRejection() { webSocketRejections.Add(1) }

func Handler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"http_requests_total":        httpRequests.Load(),
		"http_client_errors_total":   httpClientErrors.Load(),
		"http_server_errors_total":   httpServerErrors.Load(),
		"websocket_upgrades_total":   webSocketUpgrades.Load(),
		"websocket_rejections_total": webSocketRejections.Load(),
	})
}
