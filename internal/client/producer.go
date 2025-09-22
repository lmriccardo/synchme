package client

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type NotificationEvent struct {
	EventObj fsnotify.Event
}

type FileWatcherProducer struct {
	EventCh  chan<- NotificationEvent // Write-only channel for fs events
	Watcher  *fsnotify.Watcher        // The watcher for notifications on events
	FileList [](string)               // List of files watched by the Watcher
}

// CreateProducer creates a new EventProducer and returns the producer and its event channel.
// bufferSize specifies the channel buffer size; if <= 0, a default of 100 is used.
func NewProducer(buffer_size uint64) (*FileWatcherProducer, chan NotificationEvent, error) {
	if buffer_size <= 0 {
		buffer_size = 100
	}

	prod_channel := make(chan NotificationEvent, buffer_size)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, nil, err
	}

	producer := &FileWatcherProducer{prod_channel, watcher, []string{}}
	return producer, prod_channel, nil
}

func (p *FileWatcherProducer) AddPath(path string, recursive bool) {
	// Check if the input path is absolute or not
	abs_path, err := filepath.Abs(path)
	if err != nil {
		fmt.Println("Error: ", err, ". Input path will be ignored")
		return
	}

	path = abs_path

	info, err := os.Stat(path) // Take the stat of the input path
	if err != nil {
		log.Println("Error reading path:", err, ". Ingnored")
		return
	}

	if !info.IsDir() {
		// If the input path is a file we need to take the parent folder
		// as suggested in the fsnotify official documentation
		parent_dir := filepath.Dir(path)
		if err := p.Watcher.Add(parent_dir); err != nil {
			log.Println("Error adding watch:", err, ". Ignored")
		}
		p.FileList = append(p.FileList, path) // Append the path to the file list
		return
	}

	if err := p.Watcher.Add(path); err != nil {
		log.Println("Error adding watch:", err, ". Ignored")
	}

	p.FileList = append(p.FileList, path)

	// If recursive is on, then we need also to add all the subfolder
	if recursive {
		_ = filepath.WalkDir(path,
			func(subPath string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Continue reading
				}

				// Check that the subpath is not a file nor the same input path
				if !d.IsDir() || subPath == path {
					return nil
				}

				if err := p.Watcher.Add(subPath); err != nil {
					log.Println("Error adding watch:", err, ". Ignored")
				}

				p.FileList = append(p.FileList, subPath)
				return nil
			})
	}
}

// Attempt to close the watcher, otherwise log fatal the error
func (p *FileWatcherProducer) Close() {
	once := sync.Once{}
	once.Do(func() {
		if err := p.Watcher.Close(); err != nil {
			log.Println("failed to close watcher:", err)
		}
	})
}

// Produce puts the received event into the channel
func (p *FileWatcherProducer) Produce(event fsnotify.Event) {
	// Check if the file list contains the watched file or folder. Watching
	// folders also provides CREATE event handling of new children
	parentPath := filepath.Dir(event.Name)
	if slices.Contains(p.FileList, event.Name) || slices.Contains(p.FileList, parentPath) {
		p.EventCh <- NotificationEvent{event}
	}
}

// The Producer loop which produces the events
func (p *FileWatcherProducer) Run(ctx context.Context) {
	for {
		select {
		case event, ok := <-p.Watcher.Events:
			if !ok {
				return
			}
			p.Produce(event)
		case err, ok := <-p.Watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		case <-ctx.Done():
			log.Println("EventProducer canceled:", ctx.Err())
			return
		}
	}
}
