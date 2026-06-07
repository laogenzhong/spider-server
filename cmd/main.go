package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	appconfig "spider-server/common/config"
	applogger "spider-server/common/logger"
	"spider-server/game"
	"spider-server/game/appleauth"
	"spider-server/game/appstore"
	"spider-server/game/reconcile"
	"spider-server/game/session"
	"spider-server/gateway"
	mysqlconfig "spider-server/mysql"
	"sync"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := appconfig.LoadDefault()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	applogger.Configure(applogger.Config{
		Level:        cfg.Logger.Level,
		Path:         cfg.Logger.Path,
		Rotate:       cfg.Logger.Rotate,
		MaxAge:       cfg.Logger.MaxAgeDuration(),
		RotationTime: cfg.Logger.RotationTimeDuration(),
	})
	appleauth.Configure(cfg.AppleSignIn)
	appstore.Configure(cfg.AppStore)
	session.ConfigureSignSessionManager(cfg.Session.SignSecret, cfg.Session.DefaultTTLDuration())
	game.ConfigureAuth(cfg.Auth.PublicGRPCMethodPrefixes)
	game.ConfigureSign(
		cfg.Sign.Enabled,
		cfg.Sign.ReplayNonceTTLDuration(),
		cfg.Sign.ReplayNonceCleanupDuration(),
		cfg.Sign.LogMetadataPrefixOnly,
	)

	mysqlconfig.InitWithConfig(cfg.MySQL)
	reconcile.StartAppStoreReconciler(ctx, cfg.AppStore)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startGame(cfg.Server.GRPCAddr) // 这里用你已经写好的 game 启动方法
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		startGateway(cfg.Server) // 这里用你已经写好的 gateway 启动方法
	}()
	// 阻塞 main，直到收到退出信号
	<-ctx.Done()

	log.Println("receive shutdown signal")
}

func startGame(grpcAddr string) {
	grpcServer := game.NewGRPCServer(grpcAddr)
	grpcServer.Init()
	log.Println("start grpc server on", grpcAddr)

	if err := grpcServer.Start(); err != nil {
		log.Fatalf("grpc server stopped: %v", err)
	}
}

func startGateway(cfg appconfig.ServerConfig) {
	router := gateway.NewGatewayServer(cfg.GRPCTarget)

	server := &http.Server{
		Addr:              cfg.GatewayAddr,
		Handler:           router.Router(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeoutDuration(),
	}

	log.Printf("gateway server listening on %s", cfg.GatewayAddr)
	log.Printf("http  endpoint: http://%s%s/ping", cfg.EndpointHost, cfg.GatewayAddr)
	log.Printf("binary endpoint: http://%s%s/rpc", cfg.EndpointHost, cfg.GatewayAddr)
	log.Printf("ws    endpoint: ws://%s%s/ws", cfg.EndpointHost, cfg.GatewayAddr)
	log.Printf("app store notifications endpoint: http://%s%s/app-store/notifications/v2", cfg.EndpointHost, cfg.GatewayAddr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("gateway server failed: %v", err)
	}
}
