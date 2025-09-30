package server

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"google.golang.org/grpc"
)

type server struct {
	filesync.UnimplementedFileSynchServiceServer // embed for forward compatibility
}

func (s *server) Sync(stream filesync.FileSynchService_SyncServer) error {
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
	filesync.RegisterFileSynchServiceServer(grpc_server, &server{})
	log.Println("gRPC server listening on localhost:50051")

	if err := grpc_server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	fmt.Println("SynchMe Server is running")
}
