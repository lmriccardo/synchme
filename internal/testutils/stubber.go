package testutils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lmriccardo/synchme/internal/client/notification"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type Task struct {
	Fn   func(args ...string) error
	Args []string
}

type SimpleEvent struct {
	Type    notification.InternalType
	Path    string
	OldPath string
	Patches []diffmatchpatch.Patch
}

type Stubber struct {
	Ch       chan notification.NotificationEvent
	Pipeline []Task
	Expected []SimpleEvent
}

func NewStubber(ch chan notification.NotificationEvent) *Stubber {
	return &Stubber{Ch: ch, Pipeline: []Task{}, Expected: []SimpleEvent{}}
}

func (s *Stubber) AddTask(fn func(args ...string) error, args ...string) {
	s.Pipeline = append(s.Pipeline, Task{Fn: fn, Args: args})
}

func (s *Stubber) Create(path ...string) error {
	if len(path) < 1 {
		return errors.New("CreateFile needs at least one argument")
	}

	if _, err := os.Create(path[0]); err != nil {
		return err
	}

	s.Expected = append(s.Expected, SimpleEvent{
		Type: notification.Create,
		Path: path[0],
	})

	return nil
}

func (s *Stubber) Mkdir(path ...string) error {
	if len(path) < 1 {
		return errors.New("Mkdir needs at least one argument")
	}

	if err := os.MkdirAll(path[0], 0777); err != nil {
		return err
	}

	s.Expected = append(s.Expected, SimpleEvent{
		Type: notification.Create,
		Path: path[0],
	})

	return nil
}

func (s *Stubber) Remove(path ...string) error {
	if len(path) < 1 {
		return errors.New("Remove needs at least one argument")
	}

	curr_path := path[0]
	info, err := os.Stat(curr_path)

	if err != nil {
		return err
	}

	var fn func(string) error

	if info.IsDir() {
		fn = os.RemoveAll
	} else {
		fn = os.Remove
	}

	if err := fn(curr_path); err != nil {
		return err
	}

	s.Expected = append(s.Expected, SimpleEvent{
		Type: notification.Remove,
		Path: path[0],
	})

	return nil
}

func (s *Stubber) Rename(path ...string) error {
	if len(path) < 2 {
		return errors.New("Rename needs at least two arguments")
	}

	if err := os.Rename(path[0], path[1]); err != nil {
		return err
	}

	src_parent := filepath.Dir(path[0])
	dst_parent := filepath.Dir(path[1])

	var op_type notification.InternalType

	if src_parent == dst_parent {
		op_type = notification.Remove
	} else {
		op_type = notification.Move
	}

	s.Expected = append(s.Expected, SimpleEvent{
		Type:    op_type,
		Path:    path[1],
		OldPath: path[0],
	})

	return nil
}

func (s *Stubber) Write(path ...string) error {
	if len(path) < 2 {
		return errors.New("Write needs at least two arguments")
	}

	if info, err := os.Stat(path[0]); err != nil {
		return err
	} else {
		if info.IsDir() {
			return errors.New("input path must be a filename")
		}
	}

	data, err := os.ReadFile(path[0])
	if err != nil {
		return err
	}

	prev_content := string(data)
	curr_content := prev_content + path[1]

	if err := os.WriteFile(path[0], []byte(curr_content), 0777); err != nil {
		return err
	}

	dw := diffmatchpatch.New()

	s.Expected = append(s.Expected, SimpleEvent{
		Type:    notification.Write,
		Path:    path[1],
		Patches: dw.PatchMake(prev_content, curr_content),
	})

	return nil
}

func (s *Stubber) runPipeline(t *testing.T) {
	go func() {
		for _, task := range s.Pipeline {
			if err := task.Fn(task.Args...); err != nil {
				t.Error("Error when running tasks: ", err)
			}

			time.Sleep(500 * time.Millisecond)
		}
	}()
}

func (s *Stubber) Run(t *testing.T, timeout time.Duration) []notification.NotificationEvent {
	// Start the pipeline
	s.runPipeline(t)

	result := []notification.NotificationEvent{}

	// Stop collecting after timeout
	done := time.After(timeout)

collectLoop:
	for {
		select {
		case event, ok := <-s.Ch:
			if !ok {
				break collectLoop
			}
			result = append(result, event)
		case <-done:
			// Timeout reached, stop collecting
			t.Log("Timeout reached, stopping collection")
			break collectLoop
		}
	}

	return result
}
