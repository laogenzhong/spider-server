package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	appconfig "spider-server/common/config"
	"spider-server/gen/spider/api"
	rawmysqlconfig "spider-server/mysql/config"
	mysqlmodel "spider-server/mysql/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	gormlogger "gorm.io/gorm/logger"
)

const adminGrantVIPMethod = "/api.AdminVIPApi/grantVIP"

type options struct {
	configPath  string
	grpcTarget  string
	adminSecret string
	operator    string
	timeout     time.Duration
}

func main() {
	opts := parseOptions()

	cfg, err := appconfig.Load(opts.configPath)
	if err != nil {
		fatalf("load config failed: %v", err)
	}
	if opts.grpcTarget == "" {
		opts.grpcTarget = cfg.Server.GRPCTarget
	}
	if opts.adminSecret == "" {
		opts.adminSecret = cfg.Admin.VIPGrantSecret
	}
	if opts.operator == "" {
		opts.operator = defaultOperator()
	}
	if strings.TrimSpace(opts.adminSecret) == "" {
		fatalf("admin vip secret is empty; set admin.vip_grant_secret or pass -secret")
	}

	if err := initMySQL(cfg.MySQL); err != nil {
		fatalf("init mysql failed: %v", err)
	}

	conn, err := grpc.Dial(opts.grpcTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fatalf("connect grpc target %s failed: %v", opts.grpcTarget, err)
	}
	defer conn.Close()

	cli := &adminVIPCLI{
		client:      api.NewAdminVIPApiClient(conn),
		adminSecret: strings.TrimSpace(opts.adminSecret),
		operator:    strings.TrimSpace(opts.operator),
		timeout:     opts.timeout,
		reader:      bufio.NewReader(os.Stdin),
	}

	fmt.Printf("Admin VIP CLI connected to %s\n", opts.grpcTarget)
	fmt.Println("Input account like sp000001, or 0/q to exit.")
	for {
		if err := cli.runOnce(); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

type adminVIPCLI struct {
	client      api.AdminVIPApiClient
	adminSecret string
	operator    string
	timeout     time.Duration
	reader      *bufio.Reader
}

func (c *adminVIPCLI) runOnce() error {
	account, err := c.prompt("Account> ")
	if err != nil {
		return err
	}
	account = strings.TrimSpace(account)
	if isExit(account) {
		fmt.Println("Bye.")
		os.Exit(0)
	}
	if account == "" {
		return nil
	}

	user, err := mysqlmodel.GetUserByAccount(account)
	if err != nil {
		return fmt.Errorf("account %q not found or query failed: %w", account, err)
	}

	if err := c.printUserVIP(user); err != nil {
		return err
	}

	for {
		fmt.Println()
		fmt.Println("1. 开通一个月 VIP")
		fmt.Println("2. 开通一年 VIP")
		fmt.Println("3. 开通永久 VIP")
		fmt.Println("0. 退出")

		choice, err := c.prompt("Select> ")
		if err != nil {
			return err
		}
		switch strings.TrimSpace(choice) {
		case "1":
			return c.grant(account, false, 30, "admin_cli_monthly")
		case "2":
			return c.grant(account, false, 365, "admin_cli_yearly")
		case "3":
			return c.grant(account, true, 0, "admin_cli_lifetime")
		case "0", "q", "Q", "exit":
			fmt.Println("Bye.")
			os.Exit(0)
		default:
			fmt.Println("Invalid choice.")
		}
	}
}

func (c *adminVIPCLI) printUserVIP(user *mysqlmodel.User) error {
	now := time.Now()
	status, err := mysqlmodel.GetCurrentVIPStatus(uint64(user.ID), now)
	if err != nil {
		return fmt.Errorf("query vip status failed: %w", err)
	}

	fmt.Println()
	fmt.Println("User")
	fmt.Printf("  uid:      %d\n", user.ID)
	fmt.Printf("  account:  %s\n", user.Account)
	fmt.Printf("  name:     %s\n", emptyDash(userNickname(uint64(user.ID))))
	fmt.Printf("  created:  %s\n", formatTime(user.CreatedAt))
	fmt.Printf("  updated:  %s\n", formatTime(user.UpdatedAt))
	fmt.Println("VIP")
	fmt.Printf("  is_vip:   %t\n", status.IsVIP)
	fmt.Printf("  kind:     %s\n", status.Kind)
	fmt.Printf("  source:   %s\n", emptyDash(status.Source))
	fmt.Printf("  product:  %s\n", emptyDash(status.ProductID))
	fmt.Printf("  expires:  %s\n", formatTimePtr(status.ExpiresAt))
	return nil
}

func (c *adminVIPCLI) grant(account string, lifetime bool, durationDays int64, reason string) error {
	req := &api.AdminGrantVIPRequest{
		Account:      account,
		Lifetime:     lifetime,
		DurationDays: durationDays,
		Operator:     c.operator,
		Reason:       reason,
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	ctx, err := signedAdminContext(ctx, adminGrantVIPMethod, req, c.adminSecret)
	if err != nil {
		return err
	}

	var trailer metadata.MD
	resp, err := c.client.GrantVIP(ctx, req, grpc.Trailer(&trailer))
	if err != nil {
		return fmt.Errorf("grant vip rpc failed: %w", err)
	}
	if code := trailerStatusCode(trailer); code != "" && code != "0" {
		return fmt.Errorf("grant vip failed with status_code=%s", code)
	}

	fmt.Println()
	fmt.Println("Grant success")
	fmt.Printf("  uid:      %d\n", resp.GetUid())
	fmt.Printf("  account:  %s\n", resp.GetAccount())
	status := resp.GetStatus()
	if status != nil {
		fmt.Printf("  is_vip:   %t\n", status.GetIsVip())
		fmt.Printf("  kind:     %s\n", status.GetKind().String())
		fmt.Printf("  source:   %s\n", emptyDash(status.GetSource()))
		fmt.Printf("  expires:  %s\n", formatUnix(status.GetExpiresAt()))
	}
	return nil
}

func signedAdminContext(ctx context.Context, fullMethod string, msg proto.Message, adminSecret string) (context.Context, error) {
	bodyBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}
	nonce, err := newNonce()
	if err != nil {
		return nil, fmt.Errorf("generate nonce failed: %w", err)
	}

	pairs := []string{
		"xx-admin-secret", strings.TrimSpace(adminSecret),
		"xx-nonce", nonce,
		"xx-time-mills", strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	sign := sha256Hex(buildSignContent(fullMethod, pairs, bodyBytes))
	pairs = append(pairs, "xx-sign", sign)

	return metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...)), nil
}

func buildSignContent(fullMethod string, pairs []string, bodyBytes []byte) []byte {
	cleanPath := strings.TrimPrefix(strings.TrimSpace(fullMethod), "/")

	values := make(map[string][]string)
	for i := 0; i+1 < len(pairs); i += 2 {
		key := strings.ToLower(strings.TrimSpace(pairs[i]))
		if !strings.HasPrefix(key, "xx-") || key == "xx-sign" {
			continue
		}
		values[key] = append(values[key], strings.TrimSpace(pairs[i+1]))
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)

	var builder strings.Builder
	builder.WriteString(cleanPath)
	builder.WriteString("&")
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(strings.Join(values[key], ""))
		builder.WriteString("&")
	}
	builder.WriteString(sha256Hex(bodyBytes))
	return []byte(builder.String())
}

func parseOptions() options {
	var timeoutText string
	opts := options{}
	flag.StringVar(&opts.configPath, "config", "", "config file path; defaults to SPIDER_SERVER_CONFIG or config.yaml")
	flag.StringVar(&opts.grpcTarget, "grpc", "", "grpc target; defaults to server.grpc_target")
	flag.StringVar(&opts.adminSecret, "secret", "", "admin vip secret; defaults to admin.vip_grant_secret")
	flag.StringVar(&opts.operator, "operator", "", "operator name for audit; defaults to current OS user")
	flag.StringVar(&timeoutText, "timeout", "10s", "rpc timeout")
	flag.Parse()

	timeout, err := time.ParseDuration(timeoutText)
	if err != nil || timeout <= 0 {
		timeout = 10 * time.Second
	}
	opts.timeout = timeout
	return opts
}

func initMySQL(cfg appconfig.MySQLConfig) error {
	return rawmysqlconfig.InitDb(rawmysqlconfig.Config{
		User:            cfg.User,
		Password:        cfg.Password,
		Host:            cfg.Host,
		Port:            cfg.Port,
		Database:        cfg.Database,
		Charset:         cfg.Charset,
		ParseTime:       cfg.ParseTime,
		Loc:             cfg.Loc,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetimeDuration(),
		ConnMaxIdleTime: cfg.ConnMaxIdleTimeDuration(),
		LogLevel:        gormlogger.Warn,
	})
}

func trailerStatusCode(md metadata.MD) string {
	values := md.Get("status_code")
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[len(values)-1])
}

func (c *adminVIPCLI) prompt(label string) (string, error) {
	fmt.Print(label)
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
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

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func isExit(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "q", "quit", "exit":
		return true
	default:
		return false
	}
}

func defaultOperator() string {
	if user := strings.TrimSpace(os.Getenv("USER")); user != "" {
		return user
	}
	if user := strings.TrimSpace(os.Getenv("USERNAME")); user != "" {
		return user
	}
	return "admin_vip_cli"
}

func userNickname(uid uint64) string {
	profile, err := mysqlmodel.GetFriendProfileByUID(uid)
	if err != nil || profile == nil {
		return ""
	}
	return profile.Nickname
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return formatTime(*t)
}

func formatUnix(value int64) string {
	if value <= 0 {
		return "-"
	}
	return time.Unix(value, 0).Format("2006-01-02 15:04:05")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
