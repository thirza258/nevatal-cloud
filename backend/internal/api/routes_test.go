package api

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/config"
)

var updateRoutes = flag.Bool("update-routes", false, "rewrite testdata/routes.golden")

// TestRouteTableIsStable guards the public HTTP contract: every method+path the
// SPA, the WordPress plugin and MCP clients rely on must stay registered.
// Regenerate deliberately with: go test ./internal/api -run TestRouteTable -update-routes
func TestRouteTableIsStable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := SetupRouter(RouterParams{Cfg: &config.Config{Env: config.EnvDevelopment}})

	var lines []string
	for _, r := range router.Routes() {
		lines = append(lines, r.Method+" "+r.Path)
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"

	golden := filepath.Join("testdata", "routes.golden")
	if *updateRoutes {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v (run with -update-routes to create it)", golden, err)
	}
	if got != string(want) {
		t.Fatalf("route table changed.\n--- want\n%s\n--- got\n%s", want, got)
	}
}
