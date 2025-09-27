package notification

import (
	"fmt"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type InternalType int

const (
	Create InternalType = 1 << iota
	Write
	Remove
	Rename
	Move
)

func (i InternalType) Has(o InternalType) bool {
	return i&o != 0
}

func (i InternalType) String() string {
	var b strings.Builder
	if i.Has(Create) {
		b.WriteString("|CREATE")
	}
	if i.Has(Write) {
		b.WriteString("|WRITE")
	}
	if i.Has(Rename) {
		b.WriteString("|RENAME")
	}
	if i.Has(Move) {
		b.WriteString("|MOVE")
	}
	if i.Has(Remove) {
		b.WriteString("|REMOVE")
	}
	if b.Len() < 1 {
		return "[no event]"
	}
	return b.String()[1:]
}

type NotificationEvent struct {
	Path      string                 // The path to the file causing the event
	Op        InternalType           // Operation type
	Patches   []diffmatchpatch.Patch // A list of patches applied
	OldPath   string                 // Optional string representing the file old path
	Timestamp time.Time              // The timestamp of the event
}

func (ev NotificationEvent) String() string {
	return fmt.Sprintf(
		"<%v> %v at %v",
		ev.Op,
		ev.Path,
		ev.Timestamp.Format(time.RFC3339),
	)
}

type WatcherChannel struct {
	EventCh chan NotificationEvent // Producer -> Network channel
}

func NewChannel(buffer_size uint64) *WatcherChannel {
	if buffer_size <= 0 {
		buffer_size = 100
	}

	prod_channel := make(chan NotificationEvent, buffer_size)

	return &WatcherChannel{prod_channel}
}

func (ch *WatcherChannel) Close() {
	close(ch.EventCh)
}
