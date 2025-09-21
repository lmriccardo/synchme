package tests

import (
	"testing"

	"github.com/username/app-name/pkg/utils"
)

func TestToUpper(t *testing.T) {
	got := utils.ToUpper("hello")
	want := "HELLO"

	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
