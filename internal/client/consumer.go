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
	Config  *ClientConf // Client configuration structure
}

// NewConsumer creates a new FileEventConsumer on the input channel
// which accepts any kind of file operations.
func NewConsumer(pc_ch *PC_Channel, conf *ClientConf) *FileEventConsumer {
	return NewConsumerWithMask(pc_ch, conf, 0x1F)
}

// NewConsumerWithMask creates a new FileEventConsumer on the input channel
// which accepts only the input mask of operations
func NewConsumerWithMask(pc_ch *PC_Channel, conf *ClientConf, mask fsnotify.Op) *FileEventConsumer {
	consumer := &FileEventConsumer{pc_ch, mask, conf}
	consumer.Filter(fsnotify.Chmod) // Filters the chmod and write events

	// Filters also all the operations in the configuration
	for _, operation := range conf.FS_Notification.Filters {
		consumer.FilterByString(operation)
		INFO("Applied Filter: ", operation)
	}

	return consumer
}

// Filter removes the input operation from the mask
func (c *FileEventConsumer) Filter(op fsnotify.Op) {
	c.OpMask &^= op
}

func (c *FileEventConsumer) FilterByString(op string) {
	switch op {
	case "WRITE":
		c.Filter(fsnotify.Write)
	case "REMOVE":
		c.Filter(fsnotify.Remove)
	case "RENAME":
		c.Filter(fsnotify.Rename)
	case "CREATE":
		c.Filter(fsnotify.Create)
	default:
		ERROR("No operation named: ", op)
	}
}

// Allow includes an operation into the mask
func (c *FileEventConsumer) Allow(op fsnotify.Op) {
	if !c.OpMask.Has(op) {
		c.OpMask |= op
	}
}

func (c *FileEventConsumer) AllowByString(op string) {
	switch op {
	case "WRITE":
		c.Allow(fsnotify.Write)
	case "REMOVE":
		c.Allow(fsnotify.Remove)
	case "RENAME":
		c.Allow(fsnotify.Rename)
	case "CREATE":
		c.Allow(fsnotify.Create)
	default:
		ERROR("No operation named: ", op)
	}
}

// Consume consumes an event to perform some kind of operations
func (c *FileEventConsumer) Consume(event *NotificationEvent) error {
	if !c.OpMask.Has(event.EventObj.Op) {
		return nil
	}

	INFO(event)

	// Switch between possible operations
	switch event.EventObj.Op {
	case fsnotify.Write:
		// Create the diffmatchpath object
		dm := diffmatchpatch.New()
		patches := dm.PatchMake(event.PrevContent, event.CurrContent)

		if len(patches) > 0 {
			log.Println(dm.PatchToText(patches))
		}

	case fsnotify.Create:
		// A new folder or file has been created
		c.Channel.ConsumerCh <- event.EventObj.Name
	}

	return nil
}

// Run runs the infinite for loop of the event consumer
func (c *FileEventConsumer) Run(ctx context.Context) {
	for {
		select {
		case event, ok := <-c.Channel.EventCh:
			if !ok {
				INFO("Event channel closed, consumer exiting")
				return
			}

			if err := c.Consume(&event); err != nil {
				ERROR("Error: ", err)
			}
		case <-ctx.Done():
			INFO("Consumer canceled:", ctx.Err())
			return
		}
	}
}
