package inspect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestPackage creates a temporary Go package for testing.
func setupTestPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Write a go.mod
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0644)

	// Write a Go source file
	src := `// Package sample provides test utilities.
package sample

// Config holds application configuration.
type Config struct {
	// Host is the server hostname.
	Host string
	// Port is the server port.
	Port int
}

// Server handles HTTP requests.
type Server struct {
	cfg Config
}

// NewServer creates a new Server with the given config.
func NewServer(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Start starts the server.
func (s *Server) Start() error {
	return nil
}

// Stop stops the server gracefully.
func (s *Server) Stop() error {
	return nil
}

// helper is unexported and should not appear.
func helper() {}
`
	os.WriteFile(filepath.Join(dir, "sample.go"), []byte(src), 0644)

	return dir
}

func TestTree(t *testing.T) {
	dir := setupTestPackage(t)

	summaries, err := Tree(dir)
	if err != nil {
		t.Fatalf("Tree failed: %v", err)
	}

	if len(summaries) == 0 {
		t.Fatal("expected at least one package summary")
	}

	found := false
	for _, s := range summaries {
		if strings.Contains(s.ImportPath, "test") {
			found = true
			if s.Synopsis != "Package sample provides test utilities." {
				t.Errorf("unexpected synopsis: %q", s.Synopsis)
			}
		}
	}
	if !found {
		t.Error("did not find the test package in tree output")
	}
}

func TestPackage(t *testing.T) {
	dir := setupTestPackage(t)

	detail, err := Package(dir)
	if err != nil {
		t.Fatalf("Package failed: %v", err)
	}

	if detail.Name != "sample" {
		t.Errorf("expected package name 'sample', got %q", detail.Name)
	}

	if !strings.Contains(detail.Doc, "test utilities") {
		t.Errorf("expected doc to mention test utilities, got %q", detail.Doc)
	}

	// Should have Config and Server types.
	typeNames := map[string]bool{}
	for _, td := range detail.Types {
		typeNames[td.Name] = true
	}
	if !typeNames["Config"] {
		t.Error("expected Config type")
	}
	if !typeNames["Server"] {
		t.Error("expected Server type")
	}

	// Should have NewServer as a top-level or associated function.
	foundNewServer := false
	for _, f := range detail.Funcs {
		if f.Name == "NewServer" {
			foundNewServer = true
		}
	}
	for _, td := range detail.Types {
		for _, f := range td.Funcs {
			if f.Name == "NewServer" {
				foundNewServer = true
			}
		}
	}
	if !foundNewServer {
		t.Error("expected NewServer function")
	}

	// Should NOT have unexported helper.
	for _, f := range detail.Funcs {
		if f.Name == "helper" {
			t.Error("unexported function 'helper' should not be in output")
		}
	}
}

func TestSymbol(t *testing.T) {
	dir := setupTestPackage(t)

	src, err := Symbol(dir, "Config")
	if err != nil {
		t.Fatalf("Symbol failed: %v", err)
	}

	if !strings.Contains(src, "Host string") {
		t.Errorf("expected Config to contain 'Host string', got:\n%s", src)
	}
	if !strings.Contains(src, "Port int") {
		t.Errorf("expected Config to contain 'Port int', got:\n%s", src)
	}
}

func TestSymbolNotFound(t *testing.T) {
	dir := setupTestPackage(t)

	_, err := Symbol(dir, "NonExistent")
	if err == nil {
		t.Fatal("expected error for non-existent symbol")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestBody(t *testing.T) {
	dir := setupTestPackage(t)

	src, err := Body(dir, "NewServer")
	if err != nil {
		t.Fatalf("Body failed: %v", err)
	}

	if !strings.Contains(src, "func NewServer") {
		t.Errorf("expected function signature, got:\n%s", src)
	}
	if !strings.Contains(src, "return &Server{cfg: cfg}") {
		t.Errorf("expected function body, got:\n%s", src)
	}
}

func TestBodyMethod(t *testing.T) {
	dir := setupTestPackage(t)

	src, err := Body(dir, "Server.Start")
	if err != nil {
		t.Fatalf("Body (method) failed: %v", err)
	}

	if !strings.Contains(src, "func (s *Server) Start()") {
		t.Errorf("expected method signature, got:\n%s", src)
	}
}

func TestBodyNotFound(t *testing.T) {
	dir := setupTestPackage(t)

	_, err := Body(dir, "Server.NonExistent")
	if err == nil {
		t.Fatal("expected error for non-existent method")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRefs(t *testing.T) {
	dir := setupTestPackage(t)

	refs, err := Refs(dir, "Config")
	if err != nil {
		t.Fatalf("Refs failed: %v", err)
	}

	// Config is used as: type definition, field type in Server, param in NewServer.
	if len(refs) < 3 {
		t.Errorf("expected at least 3 references to Config, got %d", len(refs))
	}

	// Check that we got at least one "type" kind (the definition).
	hasType := false
	for _, r := range refs {
		if r.Kind == "type" {
			hasType = true
		}
		if r.File != "sample.go" {
			t.Errorf("expected all refs in sample.go, got %s", r.File)
		}
	}
	if !hasType {
		t.Error("expected at least one 'type' reference for Config definition")
	}
}

func TestRefsNotFound(t *testing.T) {
	dir := setupTestPackage(t)

	refs, err := Refs(dir, "NonExistentSymbol")
	if err != nil {
		t.Fatalf("Refs failed: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 refs, got %d", len(refs))
	}
}

func TestOverview(t *testing.T) {
	dir := setupTestPackage(t)

	overview, err := Overview(dir)
	if err != nil {
		t.Fatalf("Overview failed: %v", err)
	}

	if len(overview) == 0 {
		t.Fatal("expected at least one package in overview")
	}

	pkg := overview[0]
	if pkg.Synopsis != "Package sample provides test utilities." {
		t.Errorf("unexpected synopsis: %q", pkg.Synopsis)
	}

	// Should have exported types.
	typeSet := map[string]bool{}
	for _, name := range pkg.Types {
		typeSet[name] = true
	}
	if !typeSet["Config"] {
		t.Error("expected Config in overview types")
	}
	if !typeSet["Server"] {
		t.Error("expected Server in overview types")
	}

	// Should have at least NewServer in funcs.
	hasNewServer := false
	for _, sig := range pkg.Funcs {
		if strings.Contains(sig, "NewServer") {
			hasNewServer = true
		}
	}
	if !hasNewServer {
		t.Error("expected NewServer in overview funcs")
	}
}

func TestShouldSkipDir(t *testing.T) {
	tests := []struct {
		name string
		skip bool
	}{
		{".git", true},
		{".hidden", true},
		{"vendor", true},
		{"testdata", true},
		{"internal", false},
		{"pkg", false},
	}
	for _, tt := range tests {
		if got := shouldSkipDir(tt.name); got != tt.skip {
			t.Errorf("shouldSkipDir(%q) = %v, want %v", tt.name, got, tt.skip)
		}
	}
}
