package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const gatewayAddr = ":8080"

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	router := newGatewayRouter()

	server := &http.Server{
		Addr:              gatewayAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gateway server listening on %s", gatewayAddr)
	log.Printf("http  endpoint: http://127.0.0.1%s/", gatewayAddr)
	log.Printf("ws    endpoint: ws://127.0.0.1%s/ws", gatewayAddr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("gateway server failed: %v", err)
	}
}

func newGatewayRouter() *gin.Engine {
	router := gin.Default()

	// 普通 HTTP 接口。
	router.GET("/", httpHandler)

	// WebSocket 接口。
	// HTTP 和 WS 共用同一个端口，区别只在于请求路径和是否发起 Upgrade。
	router.GET("/ws", wsHandler)

	return router
}

func httpHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "http server ok",
	})
}

func wsHandler(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("upgrade websocket failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("websocket connected: %s", c.Request.RemoteAddr)

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("websocket disconnected: %s, err: %v", c.Request.RemoteAddr, err)
			return
		}

		log.Printf("websocket receive: %s", string(message))

		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Printf("websocket write failed: %v", err)
			return
		}
	}
}
