package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"spider-server/game"
	"spider-server/gateway"
	"sync"
	"syscall"
	"time"
)

const gatewayAddr = ":19080"
const grpcPort = ":18000"
const gameHost = "localhost:18000"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	defer stop()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		startGame() // 这里用你已经写好的 game 启动方法
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		startGateway() // 这里用你已经写好的 gateway 启动方法
	}()
	// 阻塞 main，直到收到退出信号
	<-ctx.Done()

	log.Println("receive shutdown signal")
}

func startGame() {
	grpcServer := game.NewGRPCServer(grpcPort)
	grpcServer.Init()
	log.Println("start grpc server on", grpcPort)

	if err := grpcServer.Start(); err != nil {
		log.Fatalf("grpc server stopped: %v", err)
	}
}

func startGateway() {
	router := gateway.NewGatewayServer(gameHost)

	server := &http.Server{
		Addr:              gatewayAddr,
		Handler:           router.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("gateway server listening on %s", gatewayAddr)
	log.Printf("http  endpoint: http://127.0.0.1%s/ping", gatewayAddr)
	log.Printf("binary endpoint: http://127.0.0.1%s/r", gatewayAddr)
	log.Printf("ws    endpoint: ws://127.0.0.1%s/ws", gatewayAddr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("gateway server failed: %v", err)
	}
}
