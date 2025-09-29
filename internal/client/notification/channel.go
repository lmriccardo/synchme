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
	Path       string                 // The path to the file causing the event
	Op         InternalType           // Operation type
	Patches    []diffmatchpatch.Patch // A list of patches applied
	OldPath    string                 // Optional string representing the file old path
	Timestamp  time.Time              // The timestamp of the event
	ReloadConf bool                   // True to reload the conf, false otherwise
	Sent       bool                   // If the notification has been sent to the sender
}

func (ev NotificationEvent) String() string {
	ts := ev.Timestamp.Format(time.RFC3339)

	switch {
	case ev.Op.Has(Create | Remove):
		return fmt.Sprintf("<%v> %v at %v", ev.Op, ev.Path, ts)
	case ev.Op.Has(Write):
		dmp := diffmatchpatch.New()

		patches := ""
		if len(ev.Patches) > 0 {
			patches = "\n" + dmp.PatchToText(ev.Patches)
		}

		return fmt.Sprintf("<%v> %v at %v%v", ev.Op, ev.Path, ts, patches)
	default:
		return fmt.Sprintf("<%v> %v -> %v at %v", ev.Op, ev.OldPath, ev.Path, ts)
	}
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
