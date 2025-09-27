package notification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/utils"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const THRESHOLD = 50 * time.Millisecond

type FileWatcher struct {
	Channel    *WatcherChannel     // Write-only channel for fs events
	InternalCh chan fsnotify.Event // A channel with all events
	Watcher    *fsnotify.Watcher   // The watcher for notifications on events
	FileList   [](string)          // List of files watched by the Watcher
	FileCache  *utils.Cache        // Maps the file with its content
	Config     *config.ClientConf  // Client configuration structure
	LastEvent  NotificationEvent   // Last produced event
	OpMask     fsnotify.Op         // Event Mask
	Flag       bool                // True if has produced, False otherwise
}

// CreateProducer creates a new EventProducer and returns the producer and its event channel.
// bufferSize specifies the channel buffer size; if <= 0, a default of 100 is used.
func NewFileWatcher(pc_ch *WatcherChannel, conf *config.ClientConf) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cache := utils.NewCache(time.Duration(conf.FS_Notification.MaxTTL) * time.Second)
	producer := &FileWatcher{
		Channel:    pc_ch,
		InternalCh: make(chan fsnotify.Event, 100),
		Watcher:    watcher,
		FileList:   []string{},
		FileCache:  cache,
		Config:     conf,
		LastEvent:  NotificationEvent{},
		Flag:       false}

	// Read the watchlist from the configuration
	watchlist := conf.FS_Notification.Paths

	// Add the configuration to the watchlist if necessary
	if *producer.Config.Config.WatchConf {
		watchlist = append(watchlist, producer.Config.Path)
		producer.FileList = []string{filepath.Dir(producer.Config.Path)}
	}

	producer.AddPaths(watchlist...)
	producer.loadCache() // Load cache with file contents

	// Load the filters
	producer.Filter(fsnotify.Chmod) // Filters the chmod and write events

	// Filters also all the operations in the configuration
	for _, operation := range conf.FS_Notification.Filters {
		producer.FilterByString(operation)
		utils.INFO("Applied Filter: ", operation)
	}

	return producer, nil
}

// Filter removes the input operation from the mask
func (c *FileWatcher) Filter(op fsnotify.Op) {
	c.OpMask &^= op
}

func (c *FileWatcher) FilterByString(op string) {
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
		utils.ERROR("No operation named: ", op)
	}
}

// Allow includes an operation into the mask
func (c *FileWatcher) Allow(op fsnotify.Op) {
	if !c.OpMask.Has(op) {
		c.OpMask |= op
	}
}

func (c *FileWatcher) AllowByString(op string) {
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
		utils.ERROR("No operation named: ", op)
	}
}

// AddPaths adds a number of paths to the current watchlist
func (p *FileWatcher) AddPaths(paths ...string) {
	for _, path := range paths {
		if subpaths := utils.WalkDir(path); subpaths != nil {
			for _, subpath := range subpaths {
				p.AddPath(subpath)
			}
		}
	}
}

func (p *FileWatcher) addToWatcher(path string) {
	if !slices.Contains(p.Watcher.WatchList(), path) {
		if err := p.Watcher.Add(path); err != nil {
			utils.ERROR("Input path will be ignored as result of error: ", err)
		} else {
			utils.INFO("Path ", path, " added to the watchlist")
		}
	}
}

// AddPath adds a single file or folder to the watchlist (non-recursively)
func (p *FileWatcher) AddPath(path string) {
	path, _ = filepath.Abs(path)
	info, _ := os.Stat(path) // Take the stat of the input path

	// Append the file to the file list
	p.FileList = append(p.FileList, path)

	if !info.IsDir() {
		// If the input path is a file we need to take the parent folder
		// as suggested in the fsnotify official documentation
		parent_dir := filepath.Dir(path)
		p.addToWatcher(parent_dir)
		return
	}

	p.addToWatcher(path)
}

func (p *FileWatcher) loadCache() {
	for _, file := range p.FileList {
		if file == p.Config.Path {
			continue
		}

		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			base_ttl := time.Duration(p.Config.FS_Notification.BaseTTL) * time.Second
			p.FileCache.Set(file, utils.ReadFileContent(file), base_ttl, true)
		}
	}
}

// Attempt to close the watcher, otherwise log fatal the error
func (p *FileWatcher) Close() {
	if err := p.Watcher.Close(); err != nil {
		utils.ERROR("Failed to close watcher: ", err)
	}
}

func (p *FileWatcher) handleCreateEvent(event *NotificationEvent) {
	// If no previous event has ever occurred then return
	if p.Flag {
		// If the difference in time is grater then the threshold returns
		diff := event.Timestamp.Sub(p.LastEvent.Timestamp)
		if diff > THRESHOLD || !p.LastEvent.Op.Has(Remove|Rename) {
			return
		}

		// Set the correct type that is either move or rename
		if p.LastEvent.Op.Has(Remove) {
			event.Op = Move
		} else {
			event.Op = Rename
		}

		event.OldPath = p.LastEvent.Path

		// Handle simple create events
		if event.Op == Create {
			p.AddPath(event.Path)
			return
		}

		// Now, if the type is either rename or remove than we need to
		// check if its previous path was the configuration file, otherwise
		// we can skip this event at all
		if check := event.Op & (Move | Rename); check != 0 {
			if event.OldPath == p.Config.Path {
				utils.INFO("Reloading configuration file from new path ", event.Path)
				p.Config.Path = event.Path
				p.FileCache.Remove(p.Config.Path) // Removes the config previously added to the cache
			}
		}
	}
}

func (p *FileWatcher) produceEvent(event fsnotify.Event, prev, curr string, time time.Time) {
	curr_event := NotificationEvent{
		Path:      event.Name,
		Op:        InternalType(event.Op),
		Patches:   []diffmatchpatch.Patch{},
		OldPath:   event.Name,
		Timestamp: time,
	}

	utils.INFO(curr_event)

	// Check the type of the event and perform releated operations
	switch event.Op {
	case fsnotify.Write:
		// Create the diffmatchpath object
		dm := diffmatchpatch.New()
		patches := dm.PatchMake(prev, curr)

		if len(patches) > 0 {
			fmt.Println(dm.PatchToText(patches))
			curr_event.Patches = patches
		}

	case fsnotify.Create:
		// A new folder or file has been created
		p.handleCreateEvent(&curr_event)
	}

	p.LastEvent = curr_event
	p.Flag = true
}

// handleFsEvent puts the received event into the channel
func (p *FileWatcher) handleFsEvent(event fsnotify.Event) {
	// Ignore the event if it filtered by the configuration filters
	if p.OpMask.Has(event.Op) {
		return
	}

	curr_time := time.Now() // Take the current time

	// Before producing events we need to check if the current event
	// is the same as previous. In that case, there must be a
	// threshold to validate that the new event should be produced or not
	if p.Flag {
		prev_name := p.LastEvent.Path
		prev_op := p.LastEvent.Op

		if prev_name == event.Name && prev_op == InternalType(event.Op) {
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

		// If it is a remove or a rename operation than we shall not try to
		// retrieve information about that file, since it now does not exists anymore.
		if event.Has(fsnotify.Remove | fsnotify.Rename) {
			// The configuration file does not exists in the cache
			if event.Name != p.Config.Path {
				p.FileCache.Remove(event.Name) // Remove the entry from the cache
			}

			p.produceEvent(event, "", "", curr_time)
			return
		}

		var prev_content string
		var curr_content string

		info, _ := os.Stat(event.Name)

		if !info.IsDir() && event.Name != p.Config.Path {
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
			curr_content = utils.ReadFileContent(event.Name)
			p.FileCache.Modify(event.Name, curr_content)
		}

		p.produceEvent(event, prev_content, curr_content, curr_time)
	}
}

func (p *FileWatcher) catchEvents() {
	for event := range p.InternalCh {
		p.handleFsEvent(event)
	}
}

// The Producer loop which produces the events
func (p *FileWatcher) Run(ctx context.Context) {
	// Starts the cache
	exp_interval := p.Config.FS_Notification.ExpirationInt
	p.FileCache.Run(ctx, time.Duration(exp_interval)*time.Second)

	// Start the go routine for catching events
	go p.catchEvents()

	for {
		select {
		case event, ok := <-p.Watcher.Events:
			// Watch for file system events
			if !ok {
				return
			}
			p.InternalCh <- event
		case err, ok := <-p.Watcher.Errors:
			// Watch for possible errors while catching events
			if !ok {
				return
			}
			utils.ERROR("Error: ", err)
		case <-ctx.Done():
			// When the context is closed then exit
			utils.INFO("EventProducer canceled: ", ctx.Err())
			return
		}
	}
}
