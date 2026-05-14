package game

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"

	"google.golang.org/grpc"
)

type GRPCServiceRegister func(server *grpc.Server)

type GRPCServer struct {
	addr      string
	server    *grpc.Server
	listener  net.Listener
	registers []GRPCServiceRegister
	mu        sync.Mutex
	started   bool
}

func NewGRPCServer(addr string, registers ...GRPCServiceRegister) *GRPCServer {
	return &GRPCServer{
		addr: addr,
		server: grpc.NewServer(
			grpc.UnaryInterceptor(authUnaryInterceptor),
		),
		registers: registers,
	}
}

func (s *GRPCServer) Register(register GRPCServiceRegister) error {
	if register == nil {
		return errors.New("grpc router register is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return errors.New("grpc server already started")
	}

	s.registers = append(s.registers, register)
	return nil
}

func (s *GRPCServer) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("grpc server already started")
	}
	s.started = true
	s.mu.Unlock()

	for _, register := range s.registers {
		if register != nil {
			register(s.server)
		}
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener

	log.Printf("grpc server listening on %s", s.addr)

	if err := s.server.Serve(listener); err != nil {
		return err
	}

	return nil
}

func (s *GRPCServer) StartAsync() {
	go func() {
		if err := s.Start(); err != nil {
			log.Printf("grpc server stopped: %v", err)
		}
	}()
}

func (s *GRPCServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	s.server.GracefulStop()
	s.started = false
}

func (s *GRPCServer) ForceStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	s.server.Stop()
	s.started = false
}

func (s *GRPCServer) WaitForShutdown(ctx context.Context) error {
	<-ctx.Done()
	s.Stop()
	return ctx.Err()
}
