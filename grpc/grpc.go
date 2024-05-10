package grpc

import (
	"fmt"
	"net"
	"sync"

	grpc "google.golang.org/grpc"
	grpc_health_server "google.golang.org/grpc/health"
	grpc_health_service "google.golang.org/grpc/health/grpc_health_v1"
	grpc_reflection "google.golang.org/grpc/reflection"
)

type GrpcSrvCfg struct {
	BindAddr string
	Port     uint16
}

// GrpcHndlr is a [SrvHndlr] for gRPC services
type GrpcHndlr struct {
	Srv *grpc.Server
	Cfg GrpcSrvCfg
}

// NewGrpcHandler Create a new [GrpcHndlr]
func NewGrpcHandler(cfg GrpcSrvCfg) *GrpcHndlr {
	return &GrpcHndlr{
		Srv: grpc.NewServer(),
		Cfg: cfg,
	}
}

// Serve defines the implementation for the [SrvHdlr] interface
func (s *GrpcHndlr) Serve(wg *sync.WaitGroup, stopch <-chan struct{}, errch chan<- error) {

	// Indicate that the work is done
	defer wg.Done()

	// Create the binding address
	baddr := fmt.Sprintf("%s:%d", s.Cfg.BindAddr, s.Cfg.Port)

	// Create the TCP socket
	list, err := net.Listen("tcp", baddr)
	if err != nil {
		errch <- err
		return
	}

	// Enable reflection
	grpc_reflection.Register(s.Srv)

	// Create the health server
	healthServer := grpc_health_server.NewServer()
	grpc_health_service.RegisterHealthServer(s.Srv, healthServer)

	go func() {
		// Listen for the signal
		<-stopch

		// Stop the server
		s.Srv.GracefulStop()
	}()

	// Start the service
	if err := s.Srv.Serve(list); err != nil {
		errch <- err
		return
	}
}

// ForceKill stop the server now
func (s *GrpcHndlr) ForceKill() {
	// Force stop the server
	s.Srv.Stop()
}
