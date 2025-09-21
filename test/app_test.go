package tests

import (
	"testing"

	utils "github.com/lmriccardo/synchme/internal/shared"
)

func TestToUpper(t *testing.T) {
	got := utils.ToUpper("hello")
	want := "HELLO"

	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}
