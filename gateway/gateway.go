package gateway

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type BinaryRPCHeader struct {
	Key   string
	Value string
}

type BinaryRPCRequest struct {
	Path    string
	Headers []BinaryRPCHeader
	Body    []byte
}

func NewGatewayServer() *gin.Engine {
	router := gin.Default()

	// 普通 HTTP JSON 接口。
	router.GET("/ping", pingHandler)

	// 普通 HTTP 二进制接口。
	router.POST("/rpc", httpHandler)

	// WebSocket 接口。
	// HTTP 和 WS 共用同一个端口，区别只在于请求路径和是否发起 Upgrade。
	router.GET("/ws", wsHandler)

	return router
}

func pingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": gin.H{
			"server": "gateway",
			"time":   time.Now().UnixMilli(),
		},
	})
}

func httpHandler(c *gin.Context) {
	requestBody, err := c.GetRawData()
	if err != nil {
		log.Printf("read rpc request body failed: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	log.Printf("http rpc receive binary request, path=%s, bytes=%d", c.Request.URL.Path, len(requestBody))

	responseBody, err := handleBinaryRPC(requestBody)
	if err != nil {
		log.Printf("handle binary rpc failed: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Data(http.StatusOK, "application/octet-stream", responseBody)
}

func handleBinaryRPC(requestBody []byte) ([]byte, error) {
	rpcRequest, err := parseBinaryRPCRequest(requestBody)
	if err != nil {
		return nil, err
	}

	// 这里直接透传业务服务器不用解析的..

	return rpcRequest.Body, nil
}

func parseBinaryRPCRequest(requestBody []byte) (*BinaryRPCRequest, error) {
	parts := bytes.SplitN(requestBody, []byte("\r"), 3)
	if len(parts) != 3 {
		return nil, errors.New("invalid binary rpc format: expected path\\rheaders\\rbody")
	}

	path := strings.TrimSpace(string(parts[0]))
	if path == "" {
		return nil, errors.New("invalid binary rpc format: empty path")
	}

	headers := parseBinaryRPCHeaders(string(parts[1]))

	return &BinaryRPCRequest{
		Path:    path,
		Headers: headers,
		Body:    parts[2],
	}, nil
}

func parseBinaryRPCHeaders(rawHeaders string) []BinaryRPCHeader {
	lines := strings.Split(rawHeaders, "\n")
	headers := make([]BinaryRPCHeader, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			headers = append(headers, BinaryRPCHeader{
				Key:   strings.TrimSpace(line),
				Value: "",
			})
			continue
		}

		headers = append(headers, BinaryRPCHeader{
			Key:   strings.TrimSpace(key),
			Value: strings.TrimSpace(value),
		})
	}

	return headers
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
