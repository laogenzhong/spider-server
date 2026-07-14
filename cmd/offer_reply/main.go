package main

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const offerCodePattern = "OfferCodeOneTimeUseCodes_*.csv"

//go:embed web/*
var webAssets embed.FS

type offerCode struct {
	Code string
	URL  string
}

type replyState struct {
	CSVHash      string `json:"csv_sha256"`
	MaxID        int    `json:"max_id"`
	GeneratedIDs []int  `json:"generated_ids,omitempty"`
}

type replyService struct {
	mu        sync.Mutex
	offers    []offerCode
	statePath string
	state     replyState
}

type replyStatus struct {
	MaxID          int `json:"max_id"`
	NextID         int `json:"next_id"`
	GeneratedCount int `json:"generated_count"`
	TotalCount     int `json:"total_count"`
}

type replyResult struct {
	ID               int    `json:"id"`
	MaxID            int    `json:"max_id"`
	NextID           int    `json:"next_id"`
	AlreadyGenerated bool   `json:"already_generated"`
	Reply            string `json:"reply"`
}

type userInputError struct {
	err error
}

func (err *userInputError) Error() string {
	return err.err.Error()
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("offer-reply", flag.ContinueOnError)
	flags.SetOutput(stdout)
	csvPathFlag := flags.String("csv", "", "兑换码 CSV 路径；默认优先查找 cmd/offer_reply 同级目录")
	statePathFlag := flags.String("state", "", "状态文件路径；默认保存在 CSV 文件旁边")
	webMode := flags.Bool("web", false, "启动本地 HTTP 服务和浏览器界面")
	listenAddress := flags.String("addr", "127.0.0.1:8787", "HTTP 服务监听地址（配合 -web 使用）")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		return errors.New("一次只能输入一个兑换序号 ID 或兑换码")
	}

	csvPath, err := resolveCSVPath(*csvPathFlag)
	if err != nil {
		return err
	}
	offers, csvHash, err := loadOfferCodes(csvPath)
	if err != nil {
		return err
	}

	statePath := strings.TrimSpace(*statePathFlag)
	if statePath == "" {
		statePath = defaultStatePath(csvPath)
	}
	state, err := loadState(statePath, csvHash)
	if err != nil {
		return err
	}
	service := &replyService{offers: offers, statePath: statePath, state: state}
	if *webMode {
		return serveHTTP(*listenAddress, service, stdout)
	}

	fmt.Fprintf(stdout, "已生成过的最大兑换序号 ID: %d\n", service.status().MaxID)

	input := ""
	if flags.NArg() == 1 {
		input = flags.Arg(0)
	} else {
		fmt.Fprint(stdout, "请输入兑换序号 ID 或兑换码: ")
		reader := bufio.NewReader(stdin)
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("读取输入: %w", readErr)
		}
		input = line
	}

	result, err := service.generate(strings.TrimSpace(input))
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "\n当前兑换序号 ID: %d\n\n%s\n\n已生成过的最大兑换序号 ID: %d\n", result.ID, result.Reply, result.MaxID)
	return nil
}

func (service *replyService) status() replyStatus {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.statusLocked()
}

func (service *replyService) statusLocked() replyStatus {
	nextID := service.state.MaxID + 1
	if nextID > len(service.offers) {
		nextID = 0
	}
	return replyStatus{
		MaxID:          service.state.MaxID,
		NextID:         nextID,
		GeneratedCount: len(service.state.GeneratedIDs),
		TotalCount:     len(service.offers),
	}
}

func (service *replyService) generate(input string) (replyResult, error) {
	id, offer, err := findOffer(input, service.offers)
	if err != nil {
		return replyResult{}, &userInputError{err: err}
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	alreadyGenerated := service.state.contains(id)
	nextState := service.state
	nextState.GeneratedIDs = append([]int(nil), service.state.GeneratedIDs...)
	nextState.record(id)
	if err := saveState(service.statePath, nextState); err != nil {
		return replyResult{}, fmt.Errorf("保存最大兑换序号 ID: %w", err)
	}
	service.state = nextState
	status := service.statusLocked()
	return replyResult{
		ID:               id,
		MaxID:            status.MaxID,
		NextID:           status.NextID,
		AlreadyGenerated: alreadyGenerated,
		Reply:            buildReply(offer),
	}, nil
}

func serveHTTP(address string, service *replyService, stdout io.Writer) error {
	handler, err := newHTTPHandler(service)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("启动 HTTP 服务: %w", err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Fprintf(stdout, "lifttags 兑换码回复服务已启动：\nhttp://%s\n按 Ctrl+C 停止服务。\n", listener.Addr().String())
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func newHTTPHandler(service *replyService) (http.Handler, error) {
	webRoot, err := fs.Sub(webAssets, "web")
	if err != nil {
		return nil, fmt.Errorf("加载网页资源: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, service.status())
	})
	mux.HandleFunc("POST /api/replies", func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(strings.ToLower(request.Header.Get("Content-Type")), "application/json") {
			writeJSON(writer, http.StatusUnsupportedMediaType, map[string]string{"error": "仅支持 application/json 请求"})
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, 2048)
		var payload struct {
			Input string `json:"input"`
		}
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "请求格式不正确"})
			return
		}
		result, err := service.generate(strings.TrimSpace(payload.Input))
		if err != nil {
			var inputErr *userInputError
			if errors.As(err, &inputErr) {
				writeJSON(writer, http.StatusBadRequest, map[string]string{"error": inputErr.Error()})
				return
			}
			writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "保存状态失败，请检查状态文件权限"})
			return
		}
		writeJSON(writer, http.StatusOK, result)
	})
	mux.Handle("GET /", http.FileServer(http.FS(webRoot)))
	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

func resolveCSVPath(explicitPath string) (string, error) {
	if path := strings.TrimSpace(explicitPath); path != "" {
		return filepath.Abs(path)
	}
	if path := strings.TrimSpace(os.Getenv("LIFTTAGS_OFFER_CODES_CSV")); path != "" {
		return filepath.Abs(path)
	}

	// Prefer files kept with this tool. When invoked from the repository root,
	// `cmd/offer_reply` is the directory containing this source file.
	searchDirs := []string{".", filepath.Join("cmd", "offer_reply")}
	if home, err := os.UserHomeDir(); err == nil {
		searchDirs = append(searchDirs, filepath.Join(home, "Desktop"))
	}

	for _, dir := range searchDirs {
		matches, err := filepath.Glob(filepath.Join(dir, offerCodePattern))
		if err != nil {
			return "", fmt.Errorf("查找兑换码 CSV: %w", err)
		}
		if len(matches) == 0 {
			continue
		}
		sort.Slice(matches, func(i, j int) bool {
			left, leftErr := os.Stat(matches[i])
			right, rightErr := os.Stat(matches[j])
			if leftErr != nil || rightErr != nil {
				return matches[i] > matches[j]
			}
			return left.ModTime().After(right.ModTime())
		})
		return filepath.Abs(matches[0])
	}
	return "", fmt.Errorf("未找到 %s；请将 CSV 放到 cmd/offer_reply，或用 -csv 指定文件路径", offerCodePattern)
}

func defaultStatePath(csvPath string) string {
	ext := filepath.Ext(csvPath)
	return strings.TrimSuffix(csvPath, ext) + ".offer-reply-state.json"
}

func loadOfferCodes(path string) ([]offerCode, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("读取兑换码 CSV %q: %w", path, err)
	}
	sum := sha256.Sum256(data)

	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = 2
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return nil, "", fmt.Errorf("解析兑换码 CSV %q: %w", path, err)
	}
	if len(records) == 0 {
		return nil, "", fmt.Errorf("兑换码 CSV %q 为空", path)
	}

	offers := make([]offerCode, 0, len(records))
	seen := make(map[string]int, len(records))
	for index, record := range records {
		code := strings.ToUpper(strings.TrimSpace(record[0]))
		url := strings.TrimSpace(record[1])
		if code == "" || url == "" {
			return nil, "", fmt.Errorf("兑换码 CSV 第 %d 行缺少兑换码或链接", index+1)
		}
		if previous, ok := seen[code]; ok {
			return nil, "", fmt.Errorf("兑换码 CSV 第 %d 行与第 %d 行兑换码重复", index+1, previous)
		}
		seen[code] = index + 1
		offers = append(offers, offerCode{Code: code, URL: url})
	}
	return offers, hex.EncodeToString(sum[:]), nil
}

func findOffer(input string, offers []offerCode) (int, offerCode, error) {
	if input == "" {
		return 0, offerCode{}, errors.New("兑换序号 ID 或兑换码不能为空")
	}
	if id, err := strconv.Atoi(input); err == nil {
		if id < 1 || id > len(offers) {
			return 0, offerCode{}, fmt.Errorf("兑换序号 ID 必须在 1 到 %d 之间", len(offers))
		}
		return id, offers[id-1], nil
	}

	code := strings.ToUpper(input)
	for index, offer := range offers {
		if offer.Code == code {
			return index + 1, offer, nil
		}
	}
	return 0, offerCode{}, fmt.Errorf("兑换码 %q 不在 CSV 中", input)
}

func loadState(path, csvHash string) (replyState, error) {
	state := replyState{CSVHash: csvHash}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return replyState{}, fmt.Errorf("读取状态文件 %q: %w", path, err)
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return replyState{}, fmt.Errorf("解析状态文件 %q: %w", path, err)
	}
	if state.CSVHash != "" && state.CSVHash != csvHash {
		return replyState{}, fmt.Errorf("状态文件 %q 属于另一份 CSV；请检查 -csv，或为新 CSV 指定新的 -state", path)
	}
	state.CSVHash = csvHash
	for _, id := range state.GeneratedIDs {
		if id > state.MaxID {
			state.MaxID = id
		}
	}
	return state, nil
}

func (state *replyState) record(id int) {
	for _, generatedID := range state.GeneratedIDs {
		if generatedID == id {
			if id > state.MaxID {
				state.MaxID = id
			}
			return
		}
	}
	state.GeneratedIDs = append(state.GeneratedIDs, id)
	sort.Ints(state.GeneratedIDs)
	if id > state.MaxID {
		state.MaxID = id
	}
}

func (state replyState) contains(id int) bool {
	for _, generatedID := range state.GeneratedIDs {
		if generatedID == id {
			return true
		}
	}
	return false
}

func saveState(path string, state replyState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".offer-reply-state-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(state); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func buildReply(offer offerCode) string {
	return fmt.Sprintf(`Hey, thanks so much for joining the lifttags giveaway! 🙌

Here’s your lifetime pro membership code: %s

You can redeem it in either of these two ways:

Option 1 – Click the link (easiest):
%s

Just open it on your iPhone and it will apply lifetime pro automatically.

Option 2 – Enter manually in the App Store:

1. Open the App Store on your iPhone
2. Tap your profile picture (top right corner)
3. Tap “Redeem Gift Card or Code”
4. Tap “You can also enter your code manually”
5. Enter %s and tap Redeem

Quick tip: if you ever see a paywall inside the app, simply tap “Restore Purchases” – your lifetime pro access will show up right away.

That’s it – enjoy tracking all your workouts in one place! 🏋️

We’re just a small team of two, and we’d really appreciate it if you could leave a rating or a short review on the App Store – it helps others discover lifttags too. 🫶

If you have any feedback or feature requests, feel free to reply anytime!`, offer.Code, offer.URL, offer.Code)
}
