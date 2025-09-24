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

type FileWatcherProducer struct {
	Channel     *PC_Channel         // Write-only channel for fs events
	Watcher     *fsnotify.Watcher   // The watcher for notifications on events
	FileList    [](string)          // List of files watched by the Watcher
	FileContent map[string](string) // Maps the file with its content
}

// CreateProducer creates a new EventProducer and returns the producer and its event channel.
// bufferSize specifies the channel buffer size; if <= 0, a default of 100 is used.
func NewProducer(pc_ch *PC_Channel) (*FileWatcherProducer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	file_map := map[string]string{}
	file_list := []string{}

	producer := &FileWatcherProducer{pc_ch, watcher, file_list, file_map}

	return producer, nil
}

// ReadFileContent reads the content of the input file
func ReadFileContent(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Impossible reading %v", path)
		return ""
	}

	return string(content)
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

	// Append the file to the file list
	p.FileList = append(p.FileList, path)

	if !info.IsDir() {
		// If the input path is a file we need to take the parent folder
		// as suggested in the fsnotify official documentation
		parent_dir := filepath.Dir(path)
		p.FileContent[path] = ReadFileContent(path)
		if err := p.Watcher.Add(parent_dir); err != nil {
			log.Println("Error adding watch:", err, ". Ignored")
		}
		return
	}

	if err := p.Watcher.Add(path); err != nil {
		log.Println("Error adding watch:", err, ". Ignored")
	}

	// If recursive is on, then we need also to add all the subfolder
	if recursive {
		_ = filepath.WalkDir(path,
			func(subPath string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Continue reading
				}

				if d.IsDir() && subPath == path {
					if err := p.Watcher.Add(subPath); err != nil {
						log.Println("Error adding watch:", err, ". Ignored")
					}

					p.FileList = append(p.FileList, subPath)
					return nil
				}

				// Check that the subpath is not the same input path
				// If the current subpath is a file then we read
				// its content and save it into the map
				if !d.IsDir() && subPath != path {
					p.FileContent[subPath] = ReadFileContent(subPath)
				}

				return nil
			},
		)
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
	info, _ := os.Stat(event.Name)

	if slices.Contains(p.FileList, event.Name) || slices.Contains(p.FileList, parentPath) {

		var prev_content string
		var curr_content string

		if !info.IsDir() {
			// Check if the current file is a key in the map. If it is not
			// contained then, a new file or folder has been created,
			// and we must insert into the map
			var ok bool
			prev_content, ok = p.FileContent[event.Name]

			if !ok {
				p.FileContent[event.Name] = ""
			}

			// Set the new content into the map
			curr_content = ReadFileContent(event.Name)
			p.FileContent[event.Name] = curr_content
		}

		p.Channel.EventCh <- NotificationEvent{event, prev_content, curr_content}
	}
}

// The Producer loop which produces the events
func (p *FileWatcherProducer) Run(ctx context.Context) {
	for {
		select {
		case event, ok := <-p.Watcher.Events:
			// Watch for file system events
			if !ok {
				return
			}
			p.Produce(event)
		case err, ok := <-p.Watcher.Errors:
			// Watch for possible errors while catching events
			if !ok {
				return
			}
			log.Println("error:", err)
		case content, ok := <-p.Channel.ConsumerCh:
			if !ok {
				return
			}
			// Recursive will always be false since it is either
			// a new file or a new folder
			p.AddPath(content, false)
		case <-ctx.Done():
			// When the context is closed then exit
			log.Println("EventProducer canceled:", ctx.Err())
			return
		}
	}
}
