package notification

import (
	"context"
	"io"
	"net"
	"strconv"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/utils"
	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type gRPC_Client struct {
	Config      *config.ClientConf              // The client configuration
	EventCh     *WatcherChannel                 // The watcher channel
	Connection  *grpc.ClientConn                // The client connection
	SyncService filesync.FileSynchServiceClient // The Synchronization client service
}

type SyncStream = grpc.BidiStreamingClient[filesync.SyncMessage, filesync.SyncMessage]

func NewClient(conf *config.ClientConf, ch *WatcherChannel) (*gRPC_Client, error) {
	server_host := conf.Network.ServerHost
	server_port := strconv.Itoa(conf.Network.ServerPort)

	// Create the new gRPC connection
	conn, err := grpc.NewClient(net.JoinHostPort(server_host, server_port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		utils.ERROR("Cannot create gRCP client for: (", server_host, ",", server_port, ")")
		return nil, err
	}

	utils.INFO("[RPC_Client] Created new gRPC client for (",
		server_host, ",", server_port, ")")

	// Create the filesync client service
	sync_service_client := filesync.NewFileSynchServiceClient(conn)

	return &gRPC_Client{Config: conf,
		EventCh:     ch,
		Connection:  conn,
		SyncService: sync_service_client}, nil
}

func (c *gRPC_Client) Close() {
	if err := c.Connection.Close(); err != nil {
		utils.ERROR("Error when closing connection: ", err)
	}
}

func (c *gRPC_Client) Run(ctx context.Context) {
	go c.fileSyncRoutine(ctx) // Start the file sync routine
	for range ctx.Done() {
		utils.INFO("[RPC_Client] gRPC Client cancelled: ", ctx.Err())
		return
	}
}

func (c *gRPC_Client) processEvent(ctx context.Context, stream *SyncStream, event *NotificationEvent) {
	// Create and send the SyncMessage to the server
	utils.INFO("[RPC_Client] Sending event: ", event)
}

func (c *gRPC_Client) fileSyncRoutine(ctx context.Context) {
	// Get the sync update stream from the Sync RPC
	sync_stream, err := c.SyncService.Sync(ctx)
	if err != nil {
		utils.ERROR("[RPC_Client] File Sync Routine Error: ", err)
		return
	}

	// Go routine for sending updates to the server
	go func() {
		utils.INFO("[RPC_Client] Starting Updates sending routine")
		for event := range c.EventCh.EventCh {
			c.processEvent(ctx, &sync_stream, &event)
		}

		_ = sync_stream.CloseSend() // Close the sending stream
	}()

	// Routine for receiving server response
	for {
		msg, err := sync_stream.Recv()
		if err == io.EOF {
			utils.WARN("[RPC_Client] Server closed stream")
			break
		}
		if err != nil {
			utils.ERROR("[RPC_Client] Receiver Error: ", err)
			break
		}

		utils.INFO("Received update: ", msg.String())
	}
}
