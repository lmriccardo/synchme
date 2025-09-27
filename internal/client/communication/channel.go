package communication

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
)

type NotificationEvent struct {
	EventObj    fsnotify.Event // The event object from the fsnotify
	PrevContent string         // The previous file content
	CurrContent string         // The current file content
	Timestamp   time.Time      // The timestamp of the event
}

func (ev *NotificationEvent) String() string {
	return fmt.Sprintf(
		"<%v> %v at %v",
		ev.EventObj.Op,
		ev.EventObj.Name,
		ev.Timestamp.Format(time.RFC3339),
	)
}

type PC_Channel struct {
	EventCh    chan NotificationEvent // Event channel from producer to consumers
	ConsumerCh chan string            // Channel in which the consumer push created folders
}

func NewChannel(buffer_size uint64) *PC_Channel {
	if buffer_size <= 0 {
		buffer_size = 100
	}

	prod_channel := make(chan NotificationEvent, buffer_size)
	cons_channel := make(chan string, buffer_size)

	return &PC_Channel{prod_channel, cons_channel}
}

func (ch *PC_Channel) Close() {
	close(ch.EventCh)
	close(ch.ConsumerCh)
}
