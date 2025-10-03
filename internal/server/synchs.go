package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"github.com/lmriccardo/synchme/internal/proto/healthcheck"
	"github.com/lmriccardo/synchme/internal/proto/session"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type server struct {
	filesync.UnimplementedFileSynchServer // embed for forward compatibility
	session.UnimplementedSessionServer    //
	healthcheck.UnimplementedHealthServer
}

func (s *server) Sync(stream filesync.FileSynch_SyncServer) error {
	// get the context
	ctx := stream.Context()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stream canceled by client")
			return ctx.Err()
		default:
			msg, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			log.Printf("Received: %v", msg)
		}
	}
}

func (s *server) Check(ctx context.Context, req *healthcheck.HealthCheckRequest) (*healthcheck.HealthCheckResponse, error) {
	return &healthcheck.HealthCheckResponse{Status: healthcheck.HealthCheckResponse_SERVING}, nil
}

func (s *server) Heartbeat(ctx context.Context, req *session.HeartbeatMsg) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func Run() {
	fmt.Println("Starting SynchMe Server ...")

	lis, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 50051,
	})

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpc_server := grpc.NewServer()
	filesync.RegisterFileSynchServer(grpc_server, &server{})
	session.RegisterSessionServer(grpc_server, &server{})
	healthcheck.RegisterHealthServer(grpc_server, &server{})

	log.Println("gRPC server listening on localhost:50051")

	if err := grpc_server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	fmt.Println("SynchMe Server is running")
}
