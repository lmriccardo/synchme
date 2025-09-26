package client

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcherProducer struct {
	Channel   *PC_Channel       // Write-only channel for fs events
	Watcher   *fsnotify.Watcher // The watcher for notifications on events
	FileList  [](string)        // List of files watched by the Watcher
	FileCache *Cache            // Maps the file with its content
	Config    *ClientConf       // Client configuration structure
	LastEvent NotificationEvent // Last produced event
	Flag      bool              // True if has produced, False otherwise
}

// CreateProducer creates a new EventProducer and returns the producer and its event channel.
// bufferSize specifies the channel buffer size; if <= 0, a default of 100 is used.
func NewProducer(pc_ch *PC_Channel, conf *ClientConf) (*FileWatcherProducer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cache := NewCache(time.Duration(conf.FS_Notification.MaxTTL) * time.Second)
	producer := &FileWatcherProducer{
		pc_ch, watcher, []string{}, cache, conf, NotificationEvent{}, false}

	// Read the watchlist from the configuration
	watchlist := conf.FS_Notification.Paths
	producer.AddPaths(watchlist...)
	producer.LoadCache() // Load cache with file contents

	// Add the configuration path if the flag is set to true
	if *producer.Config.Config.WatchConf {
		producer.AddPath(producer.Config.Path)
	}

	return producer, nil
}

// ReadFileContent reads the content of the input file
func ReadFileContent(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		WARN("Impossible reading ", path)
		return ""
	}

	return string(content)
}

// AddPaths adds a number of paths to the current watchlist
func (p *FileWatcherProducer) AddPaths(paths ...string) {
	for _, path := range paths {
		p.AddPath(path)
	}
}

func (p *FileWatcherProducer) AddPath(path string) {
	p.AddPathWithRecurse(path, *p.Config.FS_Notification.Recursive)
}

func (p *FileWatcherProducer) LoadCache() {
	for _, file := range p.FileList {
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			base_ttl := time.Duration(p.Config.FS_Notification.BaseTTL) * time.Second
			p.FileCache.Set(file, ReadFileContent(file), base_ttl, true)
		}
	}
}

func (p *FileWatcherProducer) WalkDir(path string) fs.WalkDirFunc {
	return func(subPath string, d fs.DirEntry, err error) error {
		if err != nil || subPath == path {
			return nil // Continue reading
		}

		if d.IsDir() {
			p.AddPathWithRecurse(subPath, false)
		} else {
			p.FileList = append(p.FileList, subPath)
			INFO("Path ", subPath, " added to the watchlist")
		}

		return nil
	}
}

func (p *FileWatcherProducer) AddPathWithRecurse(path string, recursive bool) {
	// Check if the input path is absolute or not
	abs_path, err := filepath.Abs(path)
	if err != nil {
		ERROR("Input path will be ignored as result of error: ", err)
		return
	}

	path = abs_path
	info, err := os.Stat(path) // Take the stat of the input path
	if err != nil {
		ERROR("Input path will be ignored as result of error: ", err)
		return
	}

	// Append the file to the file list
	p.FileList = append(p.FileList, path)

	if !info.IsDir() {
		// If the input path is a file we need to take the parent folder
		// as suggested in the fsnotify official documentation
		parent_dir := filepath.Dir(path)

		if err := p.Watcher.Add(parent_dir); err != nil {
			ERROR("Input path will be ignored as result of error: ", err)
		}
		INFO("Path ", path, " added to the watchlist")
		return
	}

	if err := p.Watcher.Add(path); err != nil {
		ERROR("Input path will be ignored as result of error: ", err)
	}

	INFO("Path ", path, " added to the watchlist")

	// If recursive is on, then we need also to add all the subfolder
	if recursive {
		_ = filepath.WalkDir(path, p.WalkDir(path))
	}
}

// Attempt to close the watcher, otherwise log fatal the error
func (p *FileWatcherProducer) Close() {
	if err := p.Watcher.Close(); err != nil {
		ERROR("Failed to close watcher: ", err)
	}
}

func (p *FileWatcherProducer) ProduceEvent(event fsnotify.Event, prev, curr string, time time.Time) {
	curr_event := NotificationEvent{event, prev, curr, time}
	p.Channel.EventCh <- curr_event
	p.LastEvent = curr_event
	p.Flag = true
}

// Produce puts the received event into the channel
func (p *FileWatcherProducer) Produce(event fsnotify.Event) {
	curr_time := time.Now() // Take the current time

	// Before producing events we need to check if the current event
	// is the same as previous. In that case, there must be a
	// threshold to validate that the new event should be produced or not
	if p.Flag {
		prev_name := p.LastEvent.EventObj.Name
		prev_op := p.LastEvent.EventObj.Op

		if prev_name == event.Name && prev_op == event.Op {
			prev_timestamp := p.LastEvent.Timestamp
			threshold := p.Config.FS_Notification.SynchInterval * float64(time.Second)
			if diff := curr_time.Sub(prev_timestamp); diff < time.Duration(threshold) {
				return
			}
		}
	}

	// Check if the file list contains the watched file or folder. Watching
	// folders also provides CREATE event handling of new children
	parentPath := filepath.Dir(event.Name)

	if slices.Contains(p.FileList, event.Name) || slices.Contains(p.FileList, parentPath) {

		// If the current event is releated to the configuration file, then it
		// automatically filters out the remove event
		if event.Name == p.Config.Path && event.Has(fsnotify.Remove) {
			WARN("Configuration file has been deleted, continuing with loaded values!!")
			return
		}

		// If it is a remove operation than we shall not try to
		// retrieve information about that file, since it now
		// does not exists anymore.
		if event.Has(fsnotify.Remove) {
			p.FileCache.Remove(event.Name) // Remove the entry from the cache
			p.ProduceEvent(event, "", "", curr_time)
			return
		}

		var prev_content string
		var curr_content string

		info, _ := os.Stat(event.Name)

		if !info.IsDir() {
			// Check if the current file is a key in the map. If it is not
			// contained then, a new file or folder has been created,
			// and we must insert into the map
			var ok bool
			cache_content, ok := p.FileCache.GetWithDefault(event.Name, "")

			if !ok {
				base_ttl := time.Duration(p.Config.FS_Notification.BaseTTL) * time.Second
				p.FileCache.Set(event.Name, cache_content, base_ttl, true)
			}

			prev_content = cache_content.(string)

			// Set the new content into the map
			curr_content = ReadFileContent(event.Name)
			p.FileCache.Modify(event.Name, curr_content)
		}

		p.ProduceEvent(event, prev_content, curr_content, curr_time)
	}
}

// The Producer loop which produces the events
func (p *FileWatcherProducer) Run(ctx context.Context) {
	// Starts the cache
	exp_interval := p.Config.FS_Notification.ExpirationInt
	p.FileCache.Run(ctx, time.Duration(exp_interval)*time.Second)

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
			ERROR("Error: ", err)
		case content, ok := <-p.Channel.ConsumerCh:
			if !ok {
				return
			}
			// Recursive will always be false since it is either
			// a new file or a new folder
			p.AddPath(content)
		case <-ctx.Done():
			// When the context is closed then exit
			INFO("EventProducer canceled: ", ctx.Err())
			return
		}
	}
}
