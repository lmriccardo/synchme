package client

import (
	"context"
	"log"

	"github.com/fsnotify/fsnotify"
)

type FileEventConsumer struct {
	EventCh <-chan NotificationEvent // Read-only channel for fs events
	OpMask  fsnotify.Op              // Event Mask
}

// NewConsumer creates a new FileEventConsumer on the input channel
// which accepts any kind of file operations.
func NewConsumer(ch <-chan NotificationEvent) *FileEventConsumer {
	return NewConsumerWithMask(ch, 0x1F)
}

// NewConsumerWithMask creates a new FileEventConsumer on the input channel
// which accepts only the input mask of operations
func NewConsumerWithMask(ch <-chan NotificationEvent, mask fsnotify.Op) *FileEventConsumer {
	return &FileEventConsumer{ch, mask}
}

// Filter removes the input operation from the mask
func (c *FileEventConsumer) Filter(op fsnotify.Op) {
	c.OpMask &^= op
}

// Allow includes an operation into the mask
func (c *FileEventConsumer) Allow(op fsnotify.Op) {
	if !c.OpMask.Has(op) {
		c.OpMask |= op
	}
}

// Consume consumes an event to perform some kind of operations
func (c *FileEventConsumer) Consume(event *NotificationEvent) error {
	if event.EventObj.Has(c.OpMask) {
		log.Println(event.EventObj)
	}

	return nil
}

// Run runs the infinite for loop of the event consumer
func (c *FileEventConsumer) Run(ctx context.Context) {
	for {
		select {
		case event, ok := <-c.EventCh:
			if !ok {
				log.Println("Event channel closed, consumer exiting")
				return
			}

			if err := c.Consume(&event); err != nil {
				log.Println(err)
			}
		case <-ctx.Done():
			log.Println("Consumer canceled:", ctx.Err())
			return
		}
	}
}
