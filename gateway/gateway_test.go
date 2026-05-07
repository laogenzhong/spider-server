package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestGatewayHTTP(t *testing.T) {
	server := httptest.NewServer(NewGatewayServer())
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("http get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected http status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body failed: %v", err)
	}

	want := `{"code":0,"message":"http server ok"}`
	if string(body) != want {
		t.Fatalf("unexpected http body: got %s, want %s", string(body), want)
	}
}

func TestGatewayWebSocket(t *testing.T) {
	server := httptest.NewServer(NewGatewayServer())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	want := "hello websocket"

	if err := conn.WriteMessage(websocket.TextMessage, []byte(want)); err != nil {
		t.Fatalf("websocket write failed: %v", err)
	}

	messageType, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("websocket read failed: %v", err)
	}

	if messageType != websocket.TextMessage {
		t.Fatalf("unexpected message type: got %d, want %d", messageType, websocket.TextMessage)
	}

	if string(message) != want {
		t.Fatalf("unexpected websocket message: got %s, want %s", string(message), want)
	}
}
