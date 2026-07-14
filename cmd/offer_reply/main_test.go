package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCSV = "FIRSTCODE123456789,https://example.com/first\nSECONDCODE12345678,https://example.com/second\n"

func TestRunFindsByIDAndPersistsMaximum(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "codes.csv")
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}

	var first bytes.Buffer
	if err := run([]string{"-csv", csvPath, "-state", statePath, "2"}, strings.NewReader(""), &first); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.String(), "当前兑换序号 ID: 2") {
		t.Fatalf("output does not contain current ID: %s", first.String())
	}
	if !strings.Contains(first.String(), "SECONDCODE12345678") || !strings.Contains(first.String(), "https://example.com/second") {
		t.Fatalf("output does not contain second offer: %s", first.String())
	}

	var second bytes.Buffer
	if err := run([]string{"-csv", csvPath, "-state", statePath, "1"}, strings.NewReader(""), &second); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(second.String(), "已生成过的最大兑换序号 ID: 2\n") {
		t.Fatalf("maximum ID was not restored: %s", second.String())
	}
	if !strings.HasSuffix(strings.TrimSpace(second.String()), "已生成过的最大兑换序号 ID: 2") {
		t.Fatalf("maximum ID unexpectedly decreased: %s", second.String())
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state replyState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.MaxID != 2 {
		t.Fatalf("MaxID = %d, want 2", state.MaxID)
	}
	if got := len(state.GeneratedIDs); got != 2 {
		t.Fatalf("len(GeneratedIDs) = %d, want 2", got)
	}
}

func TestRunFindsByCodeFromInteractiveInput(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "codes.csv")
	statePath := filepath.Join(dir, "state.json")
	if err := os.WriteFile(csvPath, []byte(testCSV), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := run([]string{"-csv", csvPath, "-state", statePath}, strings.NewReader("firstcode123456789\n"), &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "当前兑换序号 ID: 1") {
		t.Fatalf("output does not contain ID resolved from code: %s", output.String())
	}
}

func TestLoadStateRejectsDifferentCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"csv_sha256":"old","max_id":8}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadState(path, "new"); err == nil {
		t.Fatal("loadState() succeeded for a different CSV")
	}
}

func TestHTTPHandlerGeneratesReplyAndUpdatesStatus(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	service := &replyService{
		offers: []offerCode{
			{Code: "FIRSTCODE123456789", URL: "https://example.com/first"},
			{Code: "SECONDCODE12345678", URL: "https://example.com/second"},
		},
		statePath: statePath,
		state:     replyState{CSVHash: "test-hash"},
	}
	handler, err := newHTTPHandler(service)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/replies", strings.NewReader(`{"input":"2"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST /api/replies status = %d, body = %s", response.Code, response.Body.String())
	}
	var result replyResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != 2 || result.MaxID != 2 || result.Reply == "" {
		t.Fatalf("unexpected reply result: %+v", result)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusRequest)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/status status = %d", statusResponse.Code)
	}
	var status replyStatus
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.MaxID != 2 || status.GeneratedCount != 1 || status.TotalCount != 2 {
		t.Fatalf("unexpected status: %+v", status)
	}

	pageRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), "pro 回复生成器") {
		t.Fatalf("GET / did not return the web UI: status=%d", pageResponse.Code)
	}

	styleRequest := httptest.NewRequest(http.MethodGet, "/styles.css", nil)
	styleResponse := httptest.NewRecorder()
	handler.ServeHTTP(styleResponse, styleRequest)
	if styleResponse.Code != http.StatusOK || !strings.Contains(styleResponse.Body.String(), "--lime") {
		t.Fatalf("GET /styles.css did not return the embedded stylesheet: status=%d", styleResponse.Code)
	}
}
