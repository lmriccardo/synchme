package client

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/consts"
	"github.com/lmriccardo/synchme/internal/client/notification"
	"github.com/lmriccardo/synchme/internal/utils"
)

func LoadConfEnvironment() *config.ClientConf {
	// Load the environment
	config.LoadEnvironment()

	// Load the configuration
	conf_file_path := os.Getenv(consts.SYNCHME_CONFIG)
	client_conf := config.ReadConf(conf_file_path)
	return client_conf
}

func Run() {
	// Load the application environment and configuration
	client_conf := LoadConfEnvironment()
	if client_conf == nil {
		return
	}

	utils.INFO("Read configuration ", client_conf.Path)
	defer config.WriteEnvironment()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creates the producer consumer communication channel
	ch := notification.NewChannel(100)
	defer ch.Close()

	// Creates a new watcher with 0 chan buffer size
	watcher, err := notification.NewFileWatcher(ch, client_conf)
	if err != nil {
		utils.FATAL("Fatal Error: ", err)
	}

	defer watcher.Close()
	watcher.Run(ctx)

	// Creates the gRPC client for communicating with the server
	client := notification.NewClient(client_conf, ch)
	defer client.Close()
	go client.Run(ctx)

	// Create a channel to catch OS signals (CTRL+C)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	<-sigs   // Blocks until the Interrupt arrives
	cancel() // gracefully stop producer and consumer

	// Give a moment for goroutines to exit
	time.Sleep(time.Second)
}
