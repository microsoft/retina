package pktmon

import (
	"context"
	"testing"

	"github.com/microsoft/retina/pkg/config"
)

func TestStart(t *testing.T) {
	// TestStart tests the Start function.
	t.Run("TestStart", func(t *testing.T) {
		// Create a new Plugin.
		p := New(&config.Config{})
		// Start the Plugin.
		err := p.Start(context.Background())
		// Check if the error is nil.
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
	})
}
