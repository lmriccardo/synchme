package client

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creates a new producer with 0 chan buffer size
	producer, ch, err := NewProducer(0)
	if err != nil {
		log.Fatal(err)
	}

	defer producer.Close()

	producer.AddPath("/home/vscode/prova/ciao1.txt", false)
	producer.AddPath("/home/vscode/prova/prova1", true)

	// Create the consumer with the Producer Channel
	consumer := NewConsumer(ch)
	consumer.Filter(fsnotify.Chmod | fsnotify.Write) // Filters the chmod and write events

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
