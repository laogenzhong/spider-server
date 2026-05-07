package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"spider-server/gen/spider/api"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

type BinaryRPCHeader struct {
	Key   string
	Value string
}

type RequestSignFunction func(data []byte) (string, error)

type GatewayClient struct {
	baseURL    string
	httpClient *http.Client
	signFunc   RequestSignFunction
}

func NewGatewayClient(baseURL string) *GatewayClient {
	return &GatewayClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func NewGatewayClientWithSigner(baseURL string, signFunc RequestSignFunction) *GatewayClient {
	client := NewGatewayClient(baseURL)
	client.signFunc = signFunc
	return client
}

func (client *GatewayClient) CallUnary(ctx context.Context, path string, headers []BinaryRPCHeader, requestMessage proto.Message) ([]byte, error) {
	if requestMessage == nil {
		return nil, errors.New("requestMessage is nil")
	}

	protobufBody, err := proto.Marshal(requestMessage)
	if err != nil {
		return nil, fmt.Errorf("marshal request message failed: %w", err)
	}
	signedHeaders, err := client.sign(path, headers, protobufBody)
	if err != nil {
		return nil, err
	}

	requestBody, err := buildBinaryRPCPayload(path, signedHeaders, protobufBody)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/rpc", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create gateway request failed: %w", err)
	}

	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Accept", "application/octet-stream")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call gateway failed: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read gateway response failed: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway response status=%d body=%s", response.StatusCode, string(responseBody))
	}

	return responseBody, nil
}

func (client *GatewayClient) sign(path string, headers []BinaryRPCHeader, requestBytes []byte) ([]BinaryRPCHeader, error) {
	signedHeaders := make([]BinaryRPCHeader, 0, len(headers)+3)
	signedHeaders = append(signedHeaders, headers...)

	nonce, err := newNonce()
	if err != nil {
		return nil, fmt.Errorf("generate nonce failed: %w", err)
	}

	signedHeaders = append(signedHeaders,
		BinaryRPCHeader{Key: "xx-nonce", Value: nonce},
		BinaryRPCHeader{Key: "xx-time-mills", Value: strconv.FormatInt(time.Now().UnixMilli(), 10)},
	)

	canonicalBytes := buildSignContent(path, signedHeaders, requestBytes)

	var sign string
	if client.signFunc != nil {
		sign, err = client.signFunc(canonicalBytes)
		if err != nil {
			return nil, fmt.Errorf("sign request failed: %w", err)
		}
	} else {
		sign = sha256Hex(canonicalBytes)
	}

	signedHeaders = append(signedHeaders, BinaryRPCHeader{Key: "xx-sign", Value: sign})
	return signedHeaders, nil
}

func buildSignContent(path string, headers []BinaryRPCHeader, requestBytes []byte) []byte {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")

	headerValues := make(map[string][]string)
	for _, header := range headers {
		key := strings.ToLower(strings.TrimSpace(header.Key))
		if !strings.HasPrefix(key, "xx-") {
			continue
		}
		headerValues[key] = append(headerValues[key], strings.TrimSpace(header.Value))
	}

	keys := make([]string, 0, len(headerValues))
	for key := range headerValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(path)
	buffer.WriteByte('&')

	for _, key := range keys {
		buffer.WriteString(key)
		buffer.WriteByte('=')
		for _, value := range headerValues[key] {
			buffer.WriteString(value)
		}
		buffer.WriteByte('&')
	}

	buffer.WriteString(sha256Hex(requestBytes))
	return buffer.Bytes()
}

func newNonce() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func buildBinaryRPCPayload(path string, headers []BinaryRPCHeader, body []byte) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("path is empty")
	}

	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(path)
	buffer.WriteByte('\r')

	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		if key == "" {
			continue
		}

		buffer.WriteString(key)
		buffer.WriteByte(':')
		buffer.WriteString(strings.TrimSpace(header.Value))
		buffer.WriteByte('\n')
	}

	buffer.WriteByte('\r')
	buffer.Write(body)

	return buffer.Bytes(), nil
}

func exampleCallGateway() {
	gatewayClient := NewGatewayClientWithSigner("http://192.168.3.40:19080", func(data []byte) (string, error) {
		// 当前示例使用 sha256(canonicalSignContent)。
		salt := "你的salt"
		signData := append([]byte(salt), data...)

		return sha256Hex(signData), nil
	})

	responseBody, err := gatewayClient.CallUnary(
		context.Background(),
		//"/room.api.RoomSyncApi/sync",
		"/uc.SignApi/signIn",
		[]BinaryRPCHeader{},
		&api.SyncRequest{},
	)
	if err != nil {
		log.Printf("call gateway failed: %v", err)
		return
	}

	log.Printf("gateway response bytes: %d", len(responseBody))
}

func main() {
	exampleCallGateway()
}
