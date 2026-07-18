package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	appconfig "spider-server/common/config"
	applogger "spider-server/common/logger"
	"spider-server/game"
	"spider-server/game/analytics"
	"spider-server/game/appleauth"
	"spider-server/game/appstore"
	"spider-server/game/reconcile"
	"spider-server/game/session"
	"spider-server/gateway"
	mysqlconfig "spider-server/mysql"
	mysqlmodel "spider-server/mysql/model"
	"sync"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := appconfig.LoadDefault()
	if err != nil {
		applogger.Fatalf("load config failed: %v", err)
	}

	applogger.Configure(applogger.Config{
		Level:        cfg.Logger.Level,
		Path:         cfg.Logger.Path,
		ErrorPath:    cfg.Logger.ErrorPath,
		Format:       cfg.Logger.Format,
		Rotate:       cfg.Logger.Rotate,
		MaxAge:       cfg.Logger.MaxAgeDuration(),
		RotationTime: cfg.Logger.RotationTimeDuration(),
		MaxSizeMB:    cfg.Logger.MaxSizeMB,
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
	game.ConfigureWorkoutDataSync(cfg.WorkoutDataSync)
	applogger.Printf(
		"workout data sync limits gateway_request=%d rpc_request=%d snapshot_payload=%d restore_batch_snapshots=%d restore_batch_bytes=%d",
		cfg.WorkoutDataSync.GatewayMaxRequestBytes,
		cfg.WorkoutDataSync.SyncRPCMaxRequestBytes,
		cfg.WorkoutDataSync.SnapshotMaxPayloadBytes,
		cfg.WorkoutDataSync.RestoreBatchMaxSnapshots,
		cfg.WorkoutDataSync.RestoreBatchTargetBytes,
	)

	mysqlconfig.InitWithConfig(cfg.MySQL)
	analytics.StartDailyActivitySnapshotter(ctx, cfg.Admin.ActivitySnapshotAt)
	if err := mysqlmodel.SeedAppUpdateConfigFromAppConfig(cfg.AppUpdate); err != nil {
		applogger.Fatalf("seed app update config failed: %v", err)
	}
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
		startGateway(cfg) // 这里用你已经写好的 gateway 启动方法
	}()
	// 阻塞 main，直到收到退出信号
	<-ctx.Done()

	applogger.Println("receive shutdown signal")
}

func startGame(grpcAddr string) {
	grpcServer := game.NewGRPCServer(grpcAddr)
	grpcServer.Init()
	applogger.Println("start grpc server on", grpcAddr)

	if err := grpcServer.Start(); err != nil {
		applogger.Fatalf("grpc server stopped: %v", err)
	}
}

func startGateway(cfg appconfig.Config) {
	router := gateway.NewGatewayServerWithConfig(cfg.Server.GRPCTarget, cfg.Admin, cfg.WorkoutDataSync)

	server := &http.Server{
		Addr:              cfg.Server.GatewayAddr,
		Handler:           router.Router(),
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeoutDuration(),
	}

	applogger.Printf("gateway server listening on %s", cfg.Server.GatewayAddr)
	applogger.Printf("http  endpoint: http://%s%s/ping", cfg.Server.EndpointHost, cfg.Server.GatewayAddr)
	applogger.Printf("binary endpoint: http://%s%s/rpc", cfg.Server.EndpointHost, cfg.Server.GatewayAddr)
	applogger.Printf("ws    endpoint: ws://%s%s/ws", cfg.Server.EndpointHost, cfg.Server.GatewayAddr)
	applogger.Printf("app store notifications endpoint: http://%s%s/app-store/notifications/v2", cfg.Server.EndpointHost, cfg.Server.GatewayAddr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		applogger.Fatalf("gateway server failed: %v", err)
	}
}
