package notification

import (
	"context"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"github.com/lmriccardo/synchme/internal/proto/healthcheck"
	"github.com/lmriccardo/synchme/internal/proto/session"
	"github.com/lmriccardo/synchme/internal/utils"
	"github.com/sergi/go-diff/diffmatchpatch"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const CHUNK_SIZE = 512 * 1024 // 524288

type gRPC_Client struct {
	Config         *config.ClientConf              // The client configuration
	EventCh        *WatcherChannel                 // The watcher channel
	Connection     *grpc.ClientConn                // The client connection
	SyncService    filesync.FileSynchServiceClient // The Synchronization client service
	HealthService  healthcheck.HealthClient        // The healthcheck client service
	SessionService session.SessionClient           // The session client service
	ClientID       uuid.UUID                       // The client ID
	ServerActive   bool                            // If the current server is active (connection and service up)
	ctx            context.Context                 // Clear exit token
}

type SyncStream = grpc.BidiStreamingClient[filesync.SyncMessage, filesync.SyncMessage]

func NewClient(conf *config.ClientConf, ch *WatcherChannel) *gRPC_Client {
	return &gRPC_Client{Config: conf, EventCh: ch, ClientID: uuid.New(), ServerActive: false}
}

// Connect connects the client with the server specified in the configuration
func (c *gRPC_Client) Connect() error {
	server_host := c.Config.Network.ServerHost
	server_port := strconv.Itoa(c.Config.Network.ServerPort)

	// Create the new gRPC connection
	conn, err := grpc.NewClient(net.JoinHostPort(server_host, server_port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		utils.ERROR("Cannot create gRCP client for: (", server_host, ",", server_port, ")")
		return err
	}

	utils.INFO("[RPC_Client] gRPC connected to (", server_host, ",", server_port, ")")

	// Create all the client-side services
	c.SyncService = filesync.NewFileSynchServiceClient(conn)
	c.HealthService = healthcheck.NewHealthClient(conn)
	c.SessionService = session.NewSessionClient(conn)

	return nil
}

func (c *gRPC_Client) Close() {
	if err := c.Connection.Close(); err != nil {
		utils.ERROR("Error when closing connection: ", err)
	}
}

func createUpdateMessage(event *NotificationEvent, meta *filesync.MessageMeta) []*filesync.SyncMessage {
	dw := diffmatchpatch.New()
	patches_str := dw.PatchToText(event.Patches)
	chunks := []*filesync.SyncMessage{} // Initialize the chunks

	// Create the slice of chunks from the text containing patches
	data_chunks := utils.ChunkData([]byte(patches_str), CHUNK_SIZE)
	for chunk_idx, chunk_data := range data_chunks {
		chunks = append(chunks, &filesync.SyncMessage{
			Msg: &filesync.SyncMessage_Update{
				Update: &filesync.FileUpdate{
					Meta:        meta,
					Path:        event.Path,
					Data:        chunk_data,
					IsChunk:     len(data_chunks) > 1,
					ChunkIndex:  int32(chunk_idx),
					TotalChunks: int32(len(data_chunks)),
					IsFolder:    false,
				},
			},
		})
	}

	return chunks
}

func createRenameMessage(event *NotificationEvent, meta *filesync.MessageMeta) *filesync.SyncMessage {
	return &filesync.SyncMessage{Msg: &filesync.SyncMessage_Rename{
		Rename: &filesync.FileRename{Meta: meta, Path: event.Path, OldPath: event.OldPath},
	}}
}

func createRemoveMessage(event *NotificationEvent, meta *filesync.MessageMeta) *filesync.SyncMessage {
	return &filesync.SyncMessage{Msg: &filesync.SyncMessage_Remove{
		Remove: &filesync.FileRemove{Meta: meta, Path: event.Path},
	}}
}

func (c *gRPC_Client) getMsgIterator(event *NotificationEvent) func() (*filesync.SyncMessage, bool) {
	// First we need to create the metadata
	meta := filesync.MessageMeta{
		OriginClient: c.ClientID.String(),
		Version:      1,
		Timestamp:    time.Now().UnixMicro(),
	}

	messages := []*filesync.SyncMessage{}

	// Create the corresponding message for the received event operation
	switch {
	case event.Op.Has(Write | Create):
		messages = append(messages, createUpdateMessage(event, &meta)...)
	case event.Op.Has(Move | Rename):
		messages = append(messages, createRenameMessage(event, &meta))
	case event.Op.Has(Remove):
		messages = append(messages, createRemoveMessage(event, &meta))
	}

	message_idx := 0 // The message index for the iterator

	return func() (*filesync.SyncMessage, bool) {
		if message_idx >= len(messages) {
			return nil, false
		}
		curr_message := messages[message_idx]
		message_idx++
		return curr_message, true
	}
}

func (c *gRPC_Client) processEvent(stream *SyncStream, event *NotificationEvent) {
	// Create and send the SyncMessage to the server
	utils.INFO("[RPC_Client] Sending event: ", event)

	// If the connection is broken or not read yet, we cannot send anything
	if c.Connection.GetState() != connectivity.Ready {
		return
	}

	next := c.getMsgIterator(event)

	for {
		if msg, ok := next(); ok {
			if err := (*stream).Send(msg); err != nil {
				utils.ERROR("Error when seding Event: ", err)
			}
			continue
		}
		return
	}
}

func (c *gRPC_Client) syncRoutine(ctx context.Context) {
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
			c.processEvent(&sync_stream, &event)
		}

		_ = sync_stream.CloseSend() // Close the sending stream when the channel closes
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

// TODO: To be implemented
func (c *gRPC_Client) getAuthTokenFromServer() (string, error) {
	return "token", nil
}

// TODO: To be implemented
func (c *gRPC_Client) Authenticate() error {
	utils.INFO("[RPC_Client] Performing Authentication")
	_, err := c.getAuthTokenFromServer()
	if err != nil {
		utils.ERROR("[RPC_Client] Authentication failed: ", err)
		return err
	}

	return nil
}

// TODO: To be extended and enhanced
func (c *gRPC_Client) RegisterSession() error {
	return nil
}

func (c *gRPC_Client) heartbeatRoutine(interval time.Duration) {
	ticker_ch := time.Tick(interval)
	if ticker_ch == nil {
		utils.FATAL("[RPC_Client] Negative or 0 duration to heartbeat ticker")
		return
	}

	// The heartbeat message is sent to the server to notify the
	// presence of the current client. No response is needed from
	// the server, since the client will continue working.
	for {
		select {
		case curr_time := <-ticker_ch:
			// The context is still alive and the duration has passed
			// we need to send the heartbeat message to the server
			if _, err := c.SessionService.Heartbeat(c.ctx,
				&session.HeartbeatMsg{
					ClientId:  c.ClientID.String(),
					Timestamp: curr_time.UnixNano(),
				},
			); err != nil {
				utils.WARN("[RPC_Client] Heartbeat failed: ", err)
			}

		case <-c.ctx.Done():
			return
		}
	}
}

func (c *gRPC_Client) Run(ctx context.Context) {
	// Create a new context for the gRPC connection
	var cancel context.CancelFunc
	c.ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Starts the main goroutine for connection, authentication,
	// session registration and finally file synchronization.
	// If the connection is lost then a reconnection will happen
	// with a backoff time.
	go func() {
		backoff := time.Second // Initialize the backoff time to 1 second

		for {
			if err := c.Connect(); err != nil {

			}
		}
	}()

	// The input context is the overall easy exit tag
	// for the entire application, not just the context
	// for the server gRPC connection.
	for range ctx.Done() {
		utils.INFO("[RPC_Client] gRPC Client cancelled: ", ctx.Err())
		return
	}
}
