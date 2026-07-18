package gateway

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	appconfig "spider-server/common/config"
)

func TestRPCRequestBodyLimit(t *testing.T) {
	syncConfig := appconfig.Default().WorkoutDataSync
	syncConfig.GatewayMaxRequestBytes = 1024
	request := httptest.NewRequest(
		http.MethodPost,
		"/rpc",
		bytes.NewReader(make([]byte, syncConfig.GatewayMaxRequestBytes+1)),
	)
	response := httptest.NewRecorder()

	NewGatewayServerWithConfig("127.0.0.1:1", appconfig.Default().Admin, syncConfig).Router().ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}
