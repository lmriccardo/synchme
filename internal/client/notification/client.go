package notification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
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
	"google.golang.org/protobuf/types/known/emptypb"
)

const CHUNK_SIZE = 64 * 1024 // 65536

// --- gRPC Client Implementation ---

var MESSAGE_COUNTER int64 = 0

type gRPC_Client struct {
	Config         *config.ClientConf       // The client configuration
	EventCh        *WatcherChannel          // The watcher channel
	Connection     *grpc.ClientConn         // The client connection
	SyncService    filesync.FileSynchClient // The Synchronization client service
	HealthService  healthcheck.HealthClient // The healthcheck client service
	SessionService session.SessionClient    // The session client service
	ClientID       uuid.UUID                // The client ID
	Status         utils.ServerStatus       // The server status for each service
	ServerActive   atomic.Bool              // If the current server is active (connection and service up)
	ctx            context.Context          // Clear exit token
	cancel         context.CancelFunc       // Context cancellation function
	stop_ch        chan struct{}            // A channel for easy stopping of backoff
	wg             sync.WaitGroup           // Synchronization mechanism for clean exit
}

type SyncStream = grpc.BidiStreamingClient[filesync.SyncMessage, filesync.SyncMessage]

func NewClient(conf *config.ClientConf, ch *WatcherChannel) *gRPC_Client {
	return &gRPC_Client{
		Config:   conf,
		EventCh:  ch,
		ClientID: uuid.New(),
		Status: utils.ServerStatus{
			Statuses:           make(map[string]utils.ServiceStatus),
			RegisteredServices: make([]string, 0),
			Required:           make(map[string]bool),
		},
		stop_ch: make(chan struct{}),
	}
}

func (c *gRPC_Client) Close() {
	c.wg.Wait()
	_ = c.internalClose()
}

func (c *gRPC_Client) internalClose() error {
	var err error

	if c.Connection != nil {
		if err = c.Connection.Close(); err != nil {
			utils.ERROR("Error when closing connection: ", err)
		}

		c.Connection = nil
	}

	// Also ensure that the internal context is cancelled if close
	// is called outside Run's retry logic
	if c.cancel != nil {
		c.cancel()
	}

	return err
}

// CreateRpcClient connects the client with the server specified in the configuration
func (c *gRPC_Client) CreateRpcClient() error {
	utils.INFO("[RPC_Client] Attempting connection to the gRPC server")

	// Close internal connection and cancel the context
	_ = c.internalClose()

	c.ctx, c.cancel = context.WithCancel(context.Background())

	server_host := c.Config.Network.ServerHost
	server_port := strconv.Itoa(c.Config.Network.ServerPort)

	// Create the new gRPC connection
	conn, err := grpc.NewClient(
		net.JoinHostPort(server_host, server_port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return err
	}

	// Create all the client-side services
	c.Connection = conn
	c.SyncService = filesync.NewFileSynchClient(conn)
	c.HealthService = healthcheck.NewHealthClient(conn)
	c.SessionService = session.NewSessionClient(conn)

	// Registers all the service to the ServerStatus field
	if services, err := c.SessionService.Services(c.ctx, &emptypb.Empty{}); err != nil {
		_ = c.internalClose() // Close connection if we can't get the service list
		return errors.New("failed to get server service list")
	} else {
		// Clear old registrations before registering new ones
		c.Status.Clear()

		for _, service := range services.Services {
			c.Status.Register(service.Name, service.Required)
		}
	}

	// Set the initial status for each available service
	c.updateStatus()

	return nil
}

func (c *gRPC_Client) updateStatus() {
	for _, service_name := range c.Status.RegisteredServices {
		service_status, err := c.ServerHealthCheck(service_name)
		if err != nil {
			utils.ERROR("[RPC_Client] Server healthcheck failed for service ", service_name, ": ", err)
		}

		c.Status.UpdateServiceStatus(service_name, service_status)
	}

	// Store the overall service status
	c.ServerActive.Store(c.Status.GetOverallStatus() == utils.STATUS_SERVING)
}

func (c *gRPC_Client) ServerHealthCheck(service string) (utils.ServiceStatus, error) {
	resp, err := c.HealthService.Check(c.ctx, &healthcheck.HealthCheckRequest{Service: service})
	if err != nil {
		// Connection failed or HealthCheck service is down
		return utils.STATUS_UNKNOWN, err
	}

	recv_status_code := resp.GetStatus()
	if recv_status_code != healthcheck.HealthCheckResponse_SERVING {
		return utils.ServiceStatus(recv_status_code), nil
	}

	return utils.STATUS_SERVING, nil
}

func (c *gRPC_Client) healthRoutine() {
	for {
		select {
		case <-c.ctx.Done():
			return

		default:
			// Only update the status if the connection is still open
			if c.Connection != nil && c.Connection.GetState() != connectivity.Shutdown {
				c.updateStatus() // Update all service status and overall one
			} else {
				// If the connection is not active then we should force put the
				// server status to not active, otherwise we might continue
				// to send informations
				c.ServerActive.Store(false)
			}

			time.Sleep(time.Second) // Poll the status of the server every 1 second
		}
	}
}

func (c *gRPC_Client) getMsgIterator(event *NotificationEvent) func() (*filesync.SyncMessage, bool) {
	// First we need to create the metadata
	meta := filesync.MessageMeta{
		OriginClient: c.ClientID.String(),
		Version:      1,
		Timestamp:    time.Now().UnixMicro(),
		Identifier:   MESSAGE_COUNTER,
	}

	messages := []*filesync.SyncMessage{}

	// Create the corresponding message for the received event operation
	switch {
	case event.Op.Has(Write | Create):
		messages = append(messages, createUpdateMessage(event, &meta)...)
		MESSAGE_COUNTER--
	case event.Op.Has(Move | Rename):
		messages = append(messages, createRenameMessage(event, &meta))
	case event.Op.Has(Remove):
		messages = append(messages, createRemoveMessage(event, &meta))
	}

	MESSAGE_COUNTER++ // Increase the message counter
	message_idx := 0  // The message index for the iterator

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
	// Check if the server is active before sending any data
	if !c.ServerActive.Load() {
		utils.WARN("[RPC_Client] Server inactive, dropping file event: ", event.Path)
		return
	}

	// Create and send the SyncMessage to the server
	utils.INFO("[RPC_Client] Sending event: ", event)

	// Get the iterator with all the messages that needs to be sent
	next := c.getMsgIterator(event)

	for {
		if msg, ok := next(); ok {
			if err := (*stream).Send(msg); err != nil {
				utils.ERROR("Error when seding Event: ", err)
				c.cancel() // Cancel the client context to trigger a reconnection attempt
				return     // Stop processing the event
			}
			continue
		}
		return
	}
}

func (c *gRPC_Client) processReceivedMsg(msg *filesync.SyncMessage) *filesync.SyncMessage {
	// Check the arrived message. If the origin client id is the same of the
	// current id then we need to discard the message (if it is not a server
	// acknowledgment message)
	if msg.Meta.OriginClient == c.ClientID.String() {
		return nil
	}

	// Check if an Acknowledgment message is arrived
	if ack_msg, ok := msg.GetMsg().(*filesync.SyncMessage_Ack); ok {

		// Returns if the target client of the ACK is a different one
		if ack_msg.Ack.ToClient == c.ClientID.String() {
			var origin_id string

			if len(msg.Meta.OriginClient) == 0 {
				origin_id = "SERVER"
			} else {
				origin_id = msg.Meta.OriginClient
			}

			utils.INFO("Received ACK from ", origin_id, " MsgID=", msg.Meta.Identifier)
		}

		return nil
	}

	var message_path string

	// Otherwise, check if the message is Update, Remove or Rename
	switch msg.GetMsg().(type) {
	case *filesync.SyncMessage_Update:
		update_msg := msg.GetUpdate()
		message_path = update_msg.Path
		utils.INFO("Received UPDATE from ", msg.Meta.OriginClient,
			" MsgID=", msg.Meta.Identifier, " PATH=", update_msg.Path)

		// If the operation was to write content into a file, we need first
		// check that the file exists ... if it does not exists than there
		// is an error and appropriate measures must be taken (for example
		// to fetch the file from the server and then update its content ...
		// or we shall re-fetch the entire list of watched paths since the client
		// might be unsynchronized with the latest updates).
		if !utils.Exist(message_path) && update_msg.Type == filesync.FileUpdate_WRITE {
			utils.ERROR("[gRPC_Client] Path ", message_path, " does not exists!!!!")
			return nil
		}

		// If the update is to create a file
		if update_msg.Type == filesync.FileUpdate_CREATE {
			if _, err := os.Create(message_path); err != nil {
				utils.ERROR("[gRPC_Client] Error when creating ", message_path, ": ", err)
				return nil
			}

			break
		}

		// Check if the file is open in another editing application. In this case
		// there should be a configuration parameter that choose between overwriting
		// the file or merge the incoming changes with the current one.
		if utils.IsFileOpen(message_path) {
			utils.WARN("[gRPC_Client] File ", message_path, " is currently open in an application")
		}

		// First we need to take the message data containing patches in string format
		// and reconstruct the Patch type. If error, returns nil
		dw := diffmatchpatch.New()
		content := utils.ReadFileContent(message_path)
		patches, err := dw.PatchFromText(string(update_msg.Data))
		if err != nil {
			utils.ERROR("[gRPC_Client] Unable to retrieves patches from message data")
			return nil
		}

		// If it appens that some of the patches have not been applied to the current
		// file content, then we need to leave the file as it is and raise an error
		// otherwise, we can write the patched content into the file
		if patched_content, oks := dw.PatchApply(patches, content); utils.ReduceWith(utils.And, oks, true) {
			if err := os.WriteFile(message_path, []byte(patched_content), os.ModePerm); err != nil {
				utils.ERROR("[gRPC_Client] Unable to write patched content into ", message_path, ": ", err)
				return nil
			}
		} else {
			not_oks := utils.Filter(utils.ApplyToSecond[int](utils.Not), utils.Enumerate(oks))
			utils.ERROR("[gRPC_Client] Unable to apply some patches for ", message_path)
			for idx := range not_oks {
				fmt.Printf("[*] Patch Number: %v\n%v\n", idx+1, patches[idx])
			}

			return nil
		}

	case *filesync.SyncMessage_Remove:
		remove_msg := msg.GetRemove()
		message_path = remove_msg.Path
		utils.INFO("Received REMOVE from ", msg.Meta.OriginClient,
			" MsgID=", msg.Meta.Identifier, " PATH=", remove_msg.Path)

		// If the file does not exist and the operation is Remove then the current
		// client is in an unsynchronized state, since the file would have been
		// existed up to this point if synchronized
		if !utils.Exist(message_path) {
			utils.ERROR("[gRPC_Client] The path ", message_path, " has already been removed ????")
			return nil
		}

		// Call the remove function on the path specified in the message
		if err := os.RemoveAll(message_path); err != nil {
			utils.ERROR("[gRPC_Client] Error when clearing the ", message_path, " path: ", err)
			return nil
		}

	case *filesync.SyncMessage_Rename:
		rename_msg := msg.GetRename()
		message_path = rename_msg.OldPath
		utils.INFO("Received RENAME from ", msg.Meta.OriginClient,
			" MsgID=", msg.Meta.Identifier, " PATH=", rename_msg.Path)

		// If the file does not exist and the operation is Rename then the current
		// client is in an unsynchronized state, since the file would have been
		// existed up to this point if synchronized
		if !utils.Exist(message_path) {
			utils.ERROR("[gRPC_Client] The path ", message_path, " does not exists")
			return nil
		}

		// The target path shall not exist in the current version of the client
		// folder. It does not make any sense, since if a rename event has occurred
		// this means that the origin client didn't have any file named as the
		// target path, otherwise the operation would have been resulted in an error.
		if utils.Exist(rename_msg.Path) {
			utils.ERROR("[gRPC_Client] The target path ", rename_msg.Path, " already exists")
			return nil
		}

		if err := os.Rename(message_path, rename_msg.Path); err != nil {
			utils.ERROR("[gRPC_Client] Rename operation failed: ", err)
			return nil
		}
	}

	// For any received message which is not an ACK we need to send
	// the acknowledgment message to that client
	ack_meta := &filesync.MessageMeta{
		Timestamp:    time.Now().Unix(),
		OriginClient: c.ClientID.String(),
		Identifier:   msg.Meta.Identifier,
		Version:      msg.Meta.Version,
	}

	return createSyncAck(ack_meta, message_path)
}

func (c *gRPC_Client) syncRoutine() {
	// Get the sync update stream from the Sync RPC
	sync_stream, err := c.SyncService.Sync(c.ctx)
	if err != nil {
		utils.ERROR("[RPC_Client] File Sync Routine Error: ", err)
		c.cancel() // Cancel context if stream creation fails
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

	// Routine for receiving server messages (updates push or acknowledgements)
	for {
		msg, err := sync_stream.Recv()
		if err == io.EOF {
			utils.WARN("[RPC_Client] Server closed stream (EOF)")
			c.cancel() // Treat server-side stream closure as a reason to reconnect
			break
		}

		if err != nil {
			utils.ERROR("[RPC_Client] Receiver Error, cancelling client context: ", err)
			c.cancel() // Cancel context to trigger reconnection
			break
		}

		// Process the arrived message and if the result of the operation is
		// an acknowledgment message then send it to the server
		if ack_msg := c.processReceivedMsg(msg); ack_msg != nil {
			if err := sync_stream.Send(ack_msg); err != nil {
				utils.ERROR("Error when seding ACK: ", err)
				c.cancel()
				return
			}
		}
	}
}

// TODO: To be implemented
func (c *gRPC_Client) getAuthTokenFromServer() (string, error) {
	return "token", nil
}

// TODO: To be implemented
func (c *gRPC_Client) Authenticate() error {
	if !c.ServerActive.Load() {
		// Fail fast if the health routine already determined the server is unhealthy.
		return fmt.Errorf("server is not active; skipping authentication")
	}

	utils.INFO("[RPC_Client] Performing Authentication")

	_, err := c.getAuthTokenFromServer()

	// If authentication fails, we should stop proceeding and let the Run loop retry.
	if err != nil {
		utils.ERROR("[RPC_Client] Authentication failed: ", err)
		return err
	}

	return nil
}

// TODO: To be extended and enhanced
func (c *gRPC_Client) RegisterSession() error {
	if !c.ServerActive.Load() {
		// Fail fast if the health routine already determined the server is unhealthy.
		return fmt.Errorf("server is not active; skipping session registration")
	}

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
			// Here I only need that the server heartbeat service is active;
			// I do not mind if other services are down.
			if c.Status.GetServiceStatus(utils.SessionService) != utils.STATUS_SERVING {
				continue
			}

			// The context is still alive and the duration has passed
			// we need to send the heartbeat message to the server
			if _, err := c.SessionService.Heartbeat(c.ctx,
				&session.HeartbeatMsg{
					ClientId:  c.ClientID.String(),
					Timestamp: curr_time.UnixNano(),
				},
			); err != nil {
				utils.WARN("[RPC_Client] Heartbeat failed: ", err)
				c.cancel() // Cancel the context since even the heartbeat cannot be sent
			}

		case <-c.ctx.Done():
			return
		}
	}
}

func Backoff(ch chan struct{}, backoff *time.Duration) bool {
	// If client creation failed then backoff and rety
	select {
	case <-ch:
		return false // Global shutdown
	case <-time.After(*backoff):
		*backoff = min(*backoff*2, 30*time.Second)
	}

	return true
}

func (c *gRPC_Client) internalRun() {
	backoff := time.Second // Initialize the backoff time to 1 second
	heartbeat_interval := time.Duration(c.Config.Network.HeartbeatInterval) * time.Second

mainloop:
	// Step 1: Loop until we cannot establish a first connection
	for {
		// First Create a gRPC client with the connection and the context.
		// This method will set all the specific service client and the
		// context with the releated cancellation function.
		if err := c.CreateRpcClient(); err == nil {
			break // Success! Exit the connection loop
		} else {
			utils.ERROR(
				"[RPC_Client] gRPC Client creation failed. ",
				"Retrying in ", backoff, " seconds: ",
				err.Error(),
			)

			// Apply backoff operation
			if res := Backoff(c.stop_ch, &backoff); !res {
				return
			}
		}
	}

	// Step 2: Start the goroutine that updates the server status each second
	go c.healthRoutine()

	backoff = time.Second // Reset the backoff amount to 1 second

	// Step 3: Main Operational Loop: Waits for health, performs auth/reg, and starts workers.
	for {
		// A. Wait for Server Health (Blocking until c.ServerActive is true)
		for !c.ServerActive.Load() {
			// Apply backoff operation
			if res := Backoff(c.stop_ch, &backoff); !res {
				return
			}
		}

		backoff = time.Second // Reset the backoff amount to 1 second

		// B.1: Authentication of the current client
		if err := c.Authenticate(); err != nil {
			time.Sleep(backoff)
			continue
		}

		// B.2: Register the current client to a new session
		if err := c.RegisterSession(); err != nil {
			time.Sleep(backoff)
			continue
		}

		// C. Start continuous operational routines (Heartbeat and Sync)
		utils.INFO("[RPC_Client] Everything is up!! Starting operational routines.")

		// Start the heartbeat routine and the filesynch service
		go c.heartbeatRoutine(heartbeat_interval)
		go c.syncRoutine()

		// D & E. Block until the context is canceled (reconnect) OR the global
		// stop signal is received (exit). The global stop signal is the result
		// of the global client application context being closed
		select {
		case <-c.ctx.Done():
			// E. Reconnection Logic: If we reach here, c.ctx was cancelled.
			// We need to re-establish connection and start the cycle over.
			utils.WARN("[RPC_Client] Client context cancelled. Reconnecting...")

			// First, close existing old connection and try to create a new one
			_ = c.internalClose()
			time.Sleep(backoff)
			backoff = time.Second // Reset backoff for the next attempt
			goto mainloop

		case <-c.stop_ch:
			// The global application context was canceled (e.g., Ctrl+C in Run method).
			utils.INFO("[RPC_Client] Global Stop signal received. Exiting internalRun.")
			return // This will gracefully exit the internalRun goroutine
		}
	}
}

func (c *gRPC_Client) Run(ctx context.Context) {
	// Starts the main goroutine for connection, authentication, session registration and finally
	// file synchronization. If the connection is lost then a reconnection will happen
	// with a backoff time.
	go c.internalRun()

	// The input context is the overall easy exit tag for the entire application, not
	// just the context for the server gRPC connection.
	c.wg.Go(func() {
		<-ctx.Done()
		utils.INFO("[RPC_Client] Global gRPC Client cancelled: ", ctx.Err())
		c.stop_ch <- *new(struct{})
		c.cancel() // Cancel the current connection context
	})
}
