//go:build integration
// +build integration

package integration

import (
	"testing"
)

// TestServerCleanupOnFailure is a test that intentionally fails to verify server cleanup
func TestServerCleanupOnFailure(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test intentionally fails to test server cleanup
	t.Error("This test intentionally fails to verify server cleanup works")
}
