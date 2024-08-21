package workflow

import (
	"context"
	"testing"
)

func TestRunAction(t *testing.T) {
	handler, err := NewActionsHandler(context.Background(), "9f14717c-development")
	if err != nil {
		t.Errorf("NewActionsHandler() failed: %v", err)
	}
	err = handler.GetActions("webservice")
	if err != nil {
		t.Errorf("GetActions() failed: %v", err)
	}
	err = handler.RunActions()
	if err != nil {
		t.Errorf("GetActions() failed: %v", err)
	}
}