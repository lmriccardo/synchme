package client

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func Run(conf_file_path string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creates the producer consumer communication channel
	ch := NewChannel(100)
	defer ch.Close()

	// Load the configuration
	client_conf := ReadConf(conf_file_path)
	if client_conf == nil {
		return
	}

	INFO("Read configuration ", conf_file_path)

	// Creates a new producer with 0 chan buffer size
	producer, err := NewProducer(ch, client_conf)
	if err != nil {
		FATAL("Fatal Error: ", err)
	}

	defer producer.Close()

	// Create the consumer with the Producer Channel
	consumer := NewConsumer(ch, client_conf)
	consumer.Filter(fsnotify.Chmod) // Filters the chmod and write events

	go producer.Run(ctx)
	go consumer.Run(ctx)

	// Create a channel to catch OS signals (CTRL+C)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	<-sigs   // Blocks until the Interrupt arrives
	cancel() // gracefully stop producer and consumer

	// Give a moment for goroutines to exit
	time.Sleep(time.Second)
}
