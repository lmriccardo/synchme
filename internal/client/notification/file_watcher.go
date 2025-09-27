package notification

import (
	"context"
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
func NewFileWatcher(ch *WatcherChannel, conf *config.ClientConf) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cache := utils.NewCache(time.Duration(conf.FS_Notification.MaxTTL) * time.Second)
	fwatcher := &FileWatcher{
		Channel:    ch,
		InternalCh: make(chan fsnotify.Event, 100),
		Watcher:    watcher,
		FileList:   []string{},
		FileCache:  cache,
		Config:     conf,
		LastEvent:  NotificationEvent{},
		Flag:       false}

	// Prepare the when producing the file
	fwatcher.Reset()
	return fwatcher, nil
}

// Attempt to close the watcher, otherwise log fatal the error
func (fw *FileWatcher) Close() {
	if err := fw.Watcher.Close(); err != nil {
		utils.ERROR("Failed to close watcher: ", err)
	}
}

// ResetWithConf resets the entire file watcher and load a new conf
func (fw *FileWatcher) ResetWithConf(path string) {
	fw.Config = config.ReadConf(path) // load the conf
	fw.Reset()                        // Reset the watcher
}

// Reset resets the entire file watcher
func (fw *FileWatcher) Reset() {
	// Removes all watched file from the watcher
	for _, file := range fw.Watcher.WatchList() {
		_ = fw.Watcher.Remove(file)
	}

	fw.FileList = []string{}           // Empty the entire list of file of the file watcher
	fw.FileCache.EraseCache()          // Erase the entire cache
	fw.LastEvent = NotificationEvent{} // Remove the last saved event
	fw.Flag = false

	fw.loadConf() // Reload the configuration
}

// RemovePath removes a path from both the watchlist and internal list
func (fw *FileWatcher) RemovePath(path string) {
	if slices.Contains(fw.Watcher.WatchList(), path) {
		if _, err := os.Stat(path); err == nil {
			if err := fw.Watcher.Remove(path); err != nil {
				utils.ERROR("Error RemovePath: ", err)
			}
		} else if os.IsNotExist(err) {
			utils.INFO("Path already deleted, cleaning from watch list: ", path)
		} else {
			utils.ERROR("Stat error: ", err)
		}
	}

	for index, element := range fw.FileList {
		if element == path {
			fw.FileList = append(fw.FileList[:index], fw.FileList[index+1:]...)
			break
		}
	}
}

func (fw *FileWatcher) loadConf() {
	// Read the watchlist from the configuration
	watchlist := fw.Config.FS_Notification.Paths

	// Add the configuration to the watchlist if necessary
	if *fw.Config.Config.WatchConf {
		watchlist = append(watchlist, fw.Config.Path)
		fw.FileList = []string{filepath.Dir(fw.Config.Path)}
	}

	fw.AddPaths(watchlist...)
	fw.loadCache() // Load cache with file contents

	// Load the filters
	fw.Filter(fsnotify.Chmod) // Filters the chmod and write events

	// Filters also all the operations in the configuration
	for _, operation := range fw.Config.FS_Notification.Filters {
		fw.FilterByString(operation)
		utils.INFO("Applied Filter: ", operation)
	}
}

// Filter removes the input operation from the mask
func (fw *FileWatcher) Filter(op fsnotify.Op) {
	fw.OpMask &^= op
}

func (fw *FileWatcher) FilterByString(op string) {
	switch op {
	case "WRITE":
		fw.Filter(fsnotify.Write)
	case "REMOVE":
		fw.Filter(fsnotify.Remove)
	case "RENAME":
		fw.Filter(fsnotify.Rename)
	case "CREATE":
		fw.Filter(fsnotify.Create)
	default:
		utils.ERROR("No operation named: ", op)
	}
}

// Allow includes an operation into the mask
func (fw *FileWatcher) Allow(op fsnotify.Op) {
	if !fw.OpMask.Has(op) {
		fw.OpMask |= op
	}
}

func (fw *FileWatcher) AllowByString(op string) {
	switch op {
	case "WRITE":
		fw.Allow(fsnotify.Write)
	case "REMOVE":
		fw.Allow(fsnotify.Remove)
	case "RENAME":
		fw.Allow(fsnotify.Rename)
	case "CREATE":
		fw.Allow(fsnotify.Create)
	default:
		utils.ERROR("No operation named: ", op)
	}
}

// AddPaths adds a number of paths to the current watchlist
func (fw *FileWatcher) AddPaths(paths ...string) {
	for _, path := range paths {
		if subpaths := utils.WalkDir(path); subpaths != nil {
			for _, subpath := range subpaths {
				fw.AddPath(subpath)
			}
		}
	}
}

func (fw *FileWatcher) addToWatcher(path string) {
	if !slices.Contains(fw.Watcher.WatchList(), path) {
		if err := fw.Watcher.Add(path); err != nil {
			utils.ERROR("Input path will be ignored as result of error: ", err)
		} else {
			utils.INFO("Path ", path, " added to the watchlist")
		}
	}
}

// AddPath adds a single file or folder to the watchlist (non-recursively)
func (fw *FileWatcher) AddPath(path string) {
	path, _ = filepath.Abs(path)
	info, err := os.Stat(path)
	if err != nil {
		utils.ERROR("Error AddPath: ", err)
	}

	// Append the file to the file list
	fw.FileList = append(fw.FileList, path)

	if !info.IsDir() {
		// If the input path is a file we need to take the parent folder
		// as suggested in the fsnotify official documentation
		parent_dir := filepath.Dir(path)
		fw.addToWatcher(parent_dir)
		return
	}

	fw.addToWatcher(path)
}

func (fw *FileWatcher) loadCache() {
	for _, file := range fw.FileList {
		if file == fw.Config.Path {
			continue
		}

		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			base_ttl := time.Duration(fw.Config.FS_Notification.BaseTTL) * time.Second
			fw.FileCache.Set(file, utils.ReadFileContent(file), base_ttl, true)
		}
	}
}

func (fw *FileWatcher) handleCreateEvent(event *NotificationEvent) {
	// If no previous event has ever occurred then return
	if fw.Flag {
		// If the difference in time is grater then the threshold returns
		diff := event.Timestamp.Sub(fw.LastEvent.Timestamp)
		if diff > THRESHOLD || !fw.LastEvent.Op.Has(Remove|Rename) {
			return
		}

		// Set the correct type that is either move or rename
		if fw.LastEvent.Op.Has(Remove) {
			event.Op = Move
		} else {
			event.Op = Rename
		}

		event.OldPath = fw.LastEvent.Path

		// If the event is either remove or rename and the old path
		// pointed by this event was the old conf file, then we need
		// to set the Reload conf flag to true
		check := event.Op & (Move | Rename)
		event.ReloadConf = check != 0 && event.OldPath == fw.Config.Path
	}
}

func (fw *FileWatcher) applyOperations(event *NotificationEvent) {

	// If the current event cause the watcher to reload the conf
	// we need to reset the watcher
	if event.ReloadConf {
		utils.WARN("Resetting the FileWatcher and reloading conf from ", event.Path)
		fw.ResetWithConf(event.Path)
		return
	}

	// For move, remove and rename events we need to remove the old path
	if event.Op.Has(Move | Remove | Rename) {
		fw.RemovePath(event.OldPath)
	}

	// For move, rename and create we need to add the new path
	if event.Op.Has(Create | Move | Rename) {
		fw.RemovePath(event.Path)
		fw.AddPath(event.Path)
	}
}

func (fw *FileWatcher) produceEvent(event fsnotify.Event, prev, curr string, time time.Time) {

	curr_event := NotificationEvent{
		Path:       event.Name,
		Op:         InternalType(event.Op),
		Patches:    []diffmatchpatch.Patch{},
		OldPath:    event.Name,
		Timestamp:  time,
		ReloadConf: false,
	}

	was_rename := false

	// Create the diffmatchpath object
	dm := diffmatchpatch.New()

	// Check the type of the event and perform releated operations
	switch event.Op {
	case fsnotify.Write:
		patches := dm.PatchMake(prev, curr)

		if len(patches) == 0 {
			break
		}

		// Otherwise compute the patches and check if the modified
		// file is the configuration file
		curr_event.Patches = patches

		if event.Name == fw.Config.Path {
			curr_event.ReloadConf = true
		}

	case fsnotify.Create:
		// A new folder or file has been created
		fw.handleCreateEvent(&curr_event)

	case fsnotify.Rename:
		// Renames are handled as a combination of rename|remove and
		// immediately successfully create events.
		curr_event.Op = Remove
		was_rename = true
	}

	utils.INFO(curr_event)
	fw.applyOperations(&curr_event) // Apply the operations of the event

	// Reset the rename operation if necessary
	if was_rename {
		curr_event.Op = Rename
	}

	fw.Channel.EventCh <- curr_event
	fw.LastEvent = curr_event
	fw.Flag = true
}

// handleFsEvent puts the received event into the channel
func (fw *FileWatcher) handleFsEvent(event fsnotify.Event) {
	// Ignore the event if it filtered by the configuration filters
	if fw.OpMask.Has(event.Op) {
		return
	}

	curr_time := time.Now() // Take the current time

	// Before producing events we need to check if the current event
	// is the same as previous. In that case, there must be a
	// threshold to validate that the new event should be produced or not
	if fw.Flag {
		prev_name := fw.LastEvent.Path
		prev_op := fw.LastEvent.Op

		if prev_name == event.Name && prev_op == InternalType(event.Op) {
			prev_timestamp := fw.LastEvent.Timestamp
			threshold := fw.Config.FS_Notification.SynchInterval * float64(time.Second)
			if diff := curr_time.Sub(prev_timestamp); diff < time.Duration(threshold) {
				return
			}
		}
	}

	// Check if the file list contains the watched file or folder. Watching
	// folders also provides CREATE event handling of new children
	parentPath := filepath.Dir(event.Name)

	if slices.Contains(fw.FileList, event.Name) || slices.Contains(fw.FileList, parentPath) {

		// If it is a remove or a rename operation than we shall not try to
		// retrieve information about that file, since it now does not exists anymore.
		if event.Has(fsnotify.Remove | fsnotify.Rename) {
			// The configuration file does not exists in the cache
			if event.Name != fw.Config.Path {
				fw.FileCache.Remove(event.Name) // Remove the entry from the cache
			}

			fw.produceEvent(event, "", "", curr_time)
			return
		}

		info, _ := os.Stat(event.Name)

		var prev_content string
		var curr_content string

		if !info.IsDir() && event.Name != fw.Config.Path {
			// Check if the current file is a key in the map. If it is not
			// contained then, a new file or folder has been created,
			// and we must insert into the map
			var ok bool
			cache_content, ok := fw.FileCache.GetWithDefault(event.Name, "")

			if !ok {
				base_ttl := time.Duration(fw.Config.FS_Notification.BaseTTL) * time.Second
				fw.FileCache.Set(event.Name, cache_content, base_ttl, true)
			}

			prev_content = cache_content.(string)

			// Set the new content into the map
			curr_content = utils.ReadFileContent(event.Name)
			fw.FileCache.Modify(event.Name, curr_content)
		}

		fw.produceEvent(event, prev_content, curr_content, curr_time)
	}
}

func (fw *FileWatcher) catchEvents() {
	for event := range fw.InternalCh {
		fw.handleFsEvent(event)
	}
}

// The Producer loop which produces the events
func (fw *FileWatcher) Run(ctx context.Context) {
	// Starts the cache
	exp_interval := fw.Config.FS_Notification.ExpirationInt
	fw.FileCache.Run(ctx, time.Duration(exp_interval)*time.Second)

	// Start the go routine for catching events
	go fw.catchEvents()

	for {
		select {
		case event, ok := <-fw.Watcher.Events:
			// Watch for file system events
			if !ok {
				return
			}
			fw.InternalCh <- event
		case err, ok := <-fw.Watcher.Errors:
			// Watch for possible errors while catching events
			if !ok {
				return
			}
			utils.ERROR("Error 1: ", err)
		case <-ctx.Done():
			// When the context is closed then exit
			utils.INFO("EventProducer canceled: ", ctx.Err())
			return
		}
	}
}
