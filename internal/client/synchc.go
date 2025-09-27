package client

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/notification"
	"github.com/lmriccardo/synchme/internal/client/utils"
)

func Run(conf_file_path string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creates the producer consumer communication channel
	ch := notification.NewChannel(100)
	defer ch.Close()

	// Load the configuration
	client_conf := config.ReadConf(conf_file_path)
	if client_conf == nil {
		return
	}

	utils.INFO("Read configuration ", conf_file_path)

	// Creates a new producer with 0 chan buffer size
	producer, err := notification.NewFileWatcher(ch, client_conf)
	if err != nil {
		utils.FATAL("Fatal Error: ", err)
	}

	defer producer.Close()
	go producer.Run(ctx)

	// Create a channel to catch OS signals (CTRL+C)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	<-sigs   // Blocks until the Interrupt arrives
	cancel() // gracefully stop producer and consumer

	// Give a moment for goroutines to exit
	time.Sleep(time.Second)
}
