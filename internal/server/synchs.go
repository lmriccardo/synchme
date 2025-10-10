package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"github.com/lmriccardo/synchme/internal/proto/healthcheck"
	"github.com/lmriccardo/synchme/internal/proto/session"
	"github.com/lmriccardo/synchme/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var Services = map[string]bool{
	utils.FileSyncService:    true, // The file synchronization service is required
	utils.HealthCheckService: true, // Also the healthcheck service is required
	utils.SessionService:     true, // Not to mention the session service
}

type gRPC_Server struct {
	filesync.UnimplementedFileSynchServer // embed for forward compatibility
	session.UnimplementedSessionServer    //
	healthcheck.UnimplementedHealthServer
}

func (s *gRPC_Server) Sync(stream filesync.FileSynch_SyncServer) error {
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

			// Creates the ACK message and relays the update to all other clients
			if err := stream.Send(&filesync.SyncMessage{
				Meta: &filesync.MessageMeta{
					OriginClient: "",
					Timestamp:    time.Now().Unix(),
					Identifier:   msg.Meta.Identifier,
					Version:      msg.Meta.Version,
				},
				Msg: &filesync.SyncMessage_Ack{Ack: &filesync.SyncAck{
					Path:     "",
					ToClient: msg.Meta.OriginClient,
				}},
			}); err != nil {
				utils.WARN("Error when sending ACK message")
			}

			if err := stream.Send(msg); err != nil {
				utils.WARN("Error when relaying the update message")
			}
		}
	}
}

// Check returns the status of a registered service
func (s *gRPC_Server) Check(ctx context.Context, req *healthcheck.HealthCheckRequest) (*healthcheck.HealthCheckResponse, error) {
	return &healthcheck.HealthCheckResponse{Status: healthcheck.HealthCheckResponse_SERVING}, nil
}

// Heartbeat It does not returns anything, still it is used so that the client knows
// that the server is still alive (at least)
func (s *gRPC_Server) Heartbeat(ctx context.Context, req *session.HeartbeatMsg) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

// Services returns a list of all services registered to the server
func (s *gRPC_Server) Services(ctx context.Context, req *emptypb.Empty) (*session.ServicesResponse, error) {
	services := []*session.ServicesResponse_Service{}
	for service_name, required := range Services {
		services = append(services, &session.ServicesResponse_Service{
			Name:     service_name,
			Required: required,
		})
	}

	return &session.ServicesResponse{Services: services}, nil
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
	filesync.RegisterFileSynchServer(grpc_server, &gRPC_Server{})
	session.RegisterSessionServer(grpc_server, &gRPC_Server{})
	healthcheck.RegisterHealthServer(grpc_server, &gRPC_Server{})

	log.Println("gRPC server listening on localhost:50051")

	if err := grpc_server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	fmt.Println("SynchMe Server is running")
}
