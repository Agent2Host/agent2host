//go:build !integration

package adapter

import (
	"os"
	"testing"
)

func TestIntegrationReadBoundaryGuard(t *testing.T) {
	if os.Getenv("A2H_REQUIRE_INTEGRATION") == "" {
		return
	}
	t.Fatal("integration-tagged read-boundary tests were not compiled; run: go test -tags=integration ./internal/adapter/...")
}
