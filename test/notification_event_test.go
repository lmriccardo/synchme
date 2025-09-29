package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lmriccardo/synchme/internal/client/config"
	"github.com/lmriccardo/synchme/internal/client/notification"
	"github.com/lmriccardo/synchme/internal/testutils"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func TestNotification(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := notification.NewChannel(100)
	defer ch.Close()

	// Read the configuration
	abs_conf_path, _ := filepath.Abs("configs/test_config.toml")
	client_conf := config.ReadConf(abs_conf_path)

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
	inner1 := filepath.Join(folder1, "inner1")
	inner1_file := filepath.Join(inner1, "inner1_file.txt")
	inner2 := filepath.Join(inner1, "inner2")
	inner2_file := filepath.Join(inner2, "inner2_file.txt")

	// Create the stub and add all the tasks
	s := testutils.NewStubber(ch.EventCh)
	s.AddTask(s.Create, file1)                          // Create a file named file1.txt
	s.AddTask(s.Write, file1, "Hello World from File1") // Write into file1
	s.AddTask(s.Rename, file1, file1_renamed)           // Rename file1 into file1_renamed
	s.AddTask(s.Mkdir, folder1)                         // Create a folder named folder1
	s.AddTask(s.Rename, file1_renamed, file1_moved)     // Move the renamed file into the folder1
	s.AddTask(s.Mkdir, inner1)                          // Create multiple subfolders
	s.AddTask(s.Mkdir, inner2)                          // Create multiple subfolders
	s.AddTask(s.Create, inner1_file)                    // Create the inner1_file.txt
	s.AddTask(s.Create, inner2_file)                    // Create the inner2_file.txt
	s.AddTask(s.Remove, folder1)                        // Remove traces of the test

	results := s.Run(t, 9*time.Second) // Collect all the notifications
	cancel()                           // Close the context
	expected := s.Expected             // Collect the expected events

	time.Sleep(1 * time.Second)

	// The number of events must be the same
	if len(results) != len(expected) {
		t.Errorf("Expected no. events: %v != %v (generated)", len(expected), len(results))
		return
	}

	eq_ev_fn := func(ev1 notification.NotificationEvent, ev2 testutils.SimpleEvent) bool {
		dw := diffmatchpatch.New()
		return ev1.Path == ev2.Path && ev1.OldPath == ev2.OldPath &&
			len(ev1.Patches) == len(ev2.Patches) &&
			dw.PatchToText(ev1.Patches) == dw.PatchToText(ev2.Patches)
	}

	for idx := range results {
		curr_result := results[idx]
		curr_expected := expected[idx]

		// Check the current generated event and the expected one
		if curr_result.Op != curr_expected.Type {
			t.Errorf("Event no. %v: Expected Op %v != %v (generated)",
				idx+1, curr_expected.Type, curr_result.Op)
		}

		if !eq_ev_fn(curr_result, curr_expected) {
			t.Errorf("Event no. %v: The two events are dissimilar\nObtained: %v\nGenerated: %v",
				idx+1, curr_result, curr_expected)
		}
	}
}
