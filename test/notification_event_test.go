package tests

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/notification"
	"github.com/lmriccardo/synchme/internal/testutils"
)

func TestNotification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := notification.NewChannel(100)
	defer ch.Close()

	// Read the configuration
	client_conf := config.ReadConf("/workspaces/synchme/test/configs/test_config.toml")

	// Get the parent folder path
	parent_folder := client_conf.FS_Notification.Paths[0]

	// Create the FileWatcher
	fw, err := notification.NewFileWatcher(ch, client_conf)
	if err != nil {
		t.Error("Error while creating the file watcher: ", err)
	}

	go fw.Run(ctx)
	defer fw.Close()

	file1 := filepath.Join(parent_folder, "file1.txt")
	file1_renamed := filepath.Join(parent_folder, "file1_renamed.txt")
	folder1 := filepath.Join(parent_folder, "folder1")
	file1_moved := filepath.Join(folder1, "file1_renamed.txt")
	sub_folder := filepath.Join(folder1, "inner1/inner2")

	// Create the stub and add all the tasks
	s := testutils.NewStubber(ch.EventCh)
	s.AddTask(s.Create, file1)                          // Create a file named file1.txt
	s.AddTask(s.Write, file1, "Hello World from File1") // Write into file1
	s.AddTask(s.Rename, file1, file1_renamed)           // Rename file1 into file1_renamed
	s.AddTask(s.Mkdir, folder1)                         // Create a folder named folder1
	s.AddTask(s.Rename, file1_renamed, file1_moved)     // Move the renamed file into the folder1
	s.AddTask(s.Mkdir, sub_folder)                      // Create multiple subfolders
	s.AddTask(s.Remove, folder1)                        // Remove traces of the test

	results := s.Run(t, 5*time.Second) // Collect all the notifications
	cancel()                           // Close the context
	truth := s.Expected                // Collect the expected events

	// Check the events
	fmt.Println(len(results), len(truth))
}
