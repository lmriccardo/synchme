package client

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/consts"
	"github.com/lmriccardo/synchme/internal/client/notification"
	"github.com/lmriccardo/synchme/internal/utils"
)

var client_conf *config.ClientConf
var history *notification.History

func InitialSetup() error {
	// Load the environment
	config.LoadEnvironment()

	// Load the configuration
	conf_file_path := os.Getenv(consts.SYNCHME_CONFIG)
	client_conf = config.LoadConfiguration(conf_file_path)
	if client_conf == nil {
		return errors.New("")
	}

	// Load or initialize the history
	history = notification.LoadHistory(client_conf)
	if history == nil {
		return errors.New("")
	}

	return nil
}

func Run() {
	// Load the application environment and configuration
	if err := InitialSetup(); err != nil {
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
