package notification

import (
	"context"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/utils"
	"github.com/lmriccardo/synchme/internal/proto/filesync"
	"github.com/sergi/go-diff/diffmatchpatch"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

const CHUNK_SIZE = 512 * 1024

type gRPC_Client struct {
	Config      *config.ClientConf              // The client configuration
	EventCh     *WatcherChannel                 // The watcher channel
	Connection  *grpc.ClientConn                // The client connection
	SyncService filesync.FileSynchServiceClient // The Synchronization client service
	ClientID    uuid.UUID                       // The client ID
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
		SyncService: sync_service_client,
		ClientID:    uuid.New(),
	}, nil
}

func (c *gRPC_Client) Close() {
	if err := c.Connection.Close(); err != nil {
		utils.ERROR("Error when closing connection: ", err)
	}
}

func (c *gRPC_Client) Run(ctx context.Context) {
	go c.fileSyncRoutine(ctx) // Start the file sync routine
	go func() {
		for range ctx.Done() {
			utils.INFO("[RPC_Client] gRPC Client cancelled: ", ctx.Err())
			return
		}
	}()
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
			c.processEvent(&sync_stream, &event)
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
