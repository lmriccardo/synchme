package client

import (
	"context"
	"log"

	"github.com/fsnotify/fsnotify"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type FileEventConsumer struct {
	Channel *PC_Channel // Read-only channel for fs events
	OpMask  fsnotify.Op // Event Mask
}

// NewConsumer creates a new FileEventConsumer on the input channel
// which accepts any kind of file operations.
func NewConsumer(pc_ch *PC_Channel) *FileEventConsumer {
	return NewConsumerWithMask(pc_ch, 0x1F)
}

// NewConsumerWithMask creates a new FileEventConsumer on the input channel
// which accepts only the input mask of operations
func NewConsumerWithMask(pc_ch *PC_Channel, mask fsnotify.Op) *FileEventConsumer {
	return &FileEventConsumer{pc_ch, mask}
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
	if !c.OpMask.Has(event.EventObj.Op) {
		return nil
	}

	log.Println(event.EventObj)

	// Switch between possible operations
	switch event.EventObj.Op {
	case fsnotify.Write:
		// Create the diffmatchpath object
		dm := diffmatchpatch.New()
		patches := dm.PatchMake(event.PrevContent, event.CurrContent)
		log.Println(dm.PatchToText(patches))

	case fsnotify.Create:
		// A new folder or file has been created
		c.Channel.ConsumerCh <- event.EventObj.Name
	}

	// If the event is write then we need to
	return nil
}

// Run runs the infinite for loop of the event consumer
func (c *FileEventConsumer) Run(ctx context.Context) {
	for {
		select {
		case event, ok := <-c.Channel.EventCh:
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
