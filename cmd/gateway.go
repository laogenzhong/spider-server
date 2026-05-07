package main

import (
	"log"
	"net/http"
	"time"

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
	mux := http.NewServeMux()

	// 普通 HTTP 接口。
	mux.HandleFunc("/", httpHandler)

	// WebSocket 接口。
	// HTTP 和 WS 共用同一个端口，区别只在于请求路径和是否发起 Upgrade。
	mux.HandleFunc("/ws", wsHandler)

	server := &http.Server{
		Addr:              gatewayAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gateway server listening on %s", gatewayAddr)
	log.Printf("http  endpoint: http://127.0.0.1%s/", gatewayAddr)
	log.Printf("ws    endpoint: ws://127.0.0.1%s/ws", gatewayAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("gateway server failed: %v", err)
	}
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"code":0,"message":"http server ok"}`))
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade websocket failed: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("websocket connected: %s", r.RemoteAddr)

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("websocket disconnected: %s, err: %v", r.RemoteAddr, err)
			return
		}

		log.Printf("websocket receive: %s", string(message))

		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Printf("websocket write failed: %v", err)
			return
		}
	}
}
