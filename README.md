# gocontext

AST extraction CLI for progressive disclosure of Go codebases. Built for LLM agents that need to efficiently navigate large Go projects without reading every file.

## Install

```bash
go install github.com/onurattopsort/gocontext/cmd/gocontext@latest
```

Or build from source:

```bash
git clone https://github.com/onurattopsort/gocontext.git
cd gocontext
go build -o gocontext .
```

Once installed, `gocontext` works in **any** Go project — just `cd` into the project and run it.

## Commands

All commands support `--json` for structured output.

### `overview` — Full codebase snapshot

Single-call summary of every package with exported types and function signatures. Best starting point.

```
$ gocontext overview .
── github.com/onurattopsort/gocontext/cmd ──
  func Execute()

── github.com/onurattopsort/gocontext/internal/inspect ──
  Package inspect provides AST extraction utilities for Go source code.
  types:  FuncDetail, OverviewPackage, PackageDetail, PackageSummary, Reference, TypeDetail
  func Body(dir, name string) (string, error)
  func Overview(dir string) ([]OverviewPackage, error)
  func Package(dir string) (*PackageDetail, error)
  func Refs(rootDir, symbolName string) ([]Reference, error)
  ...
```

### `tree` — List all packages

```
$ gocontext tree .
github.com/onurattopsort/gocontext                   (no doc)
github.com/onurattopsort/gocontext/cmd               (no doc)
github.com/onurattopsort/gocontext/internal/inspect   Package inspect provides AST extraction utilities...
```

Skips `vendor`, `testdata`, and hidden directories automatically.

### `package` — Inspect exported symbols

```
$ gocontext package ./internal/inspect
package inspect
import "github.com/onurattopsort/gocontext/internal/inspect"

Package inspect provides AST extraction utilities for Go source code.

── struct PackageDetail ──
PackageDetail holds full extracted documentation for a package.

  func Package(dir string) (*PackageDetail, error)
    ...
```

Accepts filesystem paths (`./pkg/foo`) or full import paths (`github.com/org/repo/pkg/foo`).

### `symbol` — Get a type definition

```
$ gocontext symbol ./internal/inspect PackageDetail
type PackageDetail struct {
	Name       string       `json:"name"`
	ImportPath string       `json:"import_path"`
	Doc        string       `json:"doc,omitempty"`
	Funcs      []FuncDetail `json:"funcs,omitempty"`
	Types      []TypeDetail `json:"types,omitempty"`
}
```

### `body` — Get function/method source

```
$ gocontext body ./internal/inspect ResolveImportPath
func ResolveImportPath(input string) (string, error) {
	// ... full implementation
}
```

For methods, use `Type.Method` syntax:

```
$ gocontext body ./pkg/server Server.HandleRequest
```

### `refs` — Find all references to a symbol

```
$ gocontext refs . ResolveImportPath
References to "ResolveImportPath" (4 found):

  cmd/root.go:66:23 [call]
    dir, err := inspect.ResolveImportPath(args[0])

  cmd/root.go:127:23 [call]
    dir, err := inspect.ResolveImportPath(args[0])

  internal/inspect/inspect.go:681:6 [decl]
    func ResolveImportPath(input string) (string, error) {
```

Each reference includes file, line, column, a kind classification (`call`, `type`, `field`, `assign`, `decl`, `ref`), and the source line for context.

### JSON output

Add `--json` to any command for structured output:

```
$ gocontext refs . Config --json
[
  {
    "file": "internal/inspect/inspect.go",
    "line": 22,
    "column": 6,
    "context": "type Config struct {",
    "kind": "type"
  },
  ...
]
```

## Progressive Disclosure Workflow

The commands are designed for a drill-down workflow:

```
overview .                      → full codebase snapshot in one call
tree .                          → see all packages
package ./internal/inspect      → see what a package exports
symbol ./internal/inspect Foo   → read the Foo type definition
body ./internal/inspect Foo.Bar → read the Bar method implementation
refs . Foo                      → find everywhere Foo is used
```

This lets an LLM agent navigate a large codebase token-efficiently, only pulling in the code it needs.

## Claude Code Integration

### Automatic agent usage (recommended)

Add the gocontext snippet to your project's `CLAUDE.md` so the agent uses it proactively without being asked:

```bash
# In your Go project root
cat >> CLAUDE.md < <(curl -sL https://raw.githubusercontent.com/onurattopsort/gocontext/main/claude-md-snippet.md)
```

Or manually copy the contents of [`claude-md-snippet.md`](claude-md-snippet.md) into your project's `CLAUDE.md`.

This tells Claude Code to prefer `gocontext` for structural queries (types, functions, references) and fall back to grep/read for text searches.

### Slash command

Optionally install the skill for explicit `/gocontext` invocation:

```bash
mkdir -p ~/.claude/skills/gocontext
cp .claude/skills/gocontext/SKILL.md ~/.claude/skills/gocontext/SKILL.md
```

Then in any Go project:

```
/gocontext overview .
/gocontext package ./pkg/server
/gocontext refs . Config
```

## How It Works

- Uses `go/parser` and `go/ast` to parse Go source files
- Uses `go/doc` to extract documentation and organize symbols
- Uses `go/token.FileSet` to map AST positions back to source bytes for exact extraction
- Resolves import paths via `go.mod` module declarations
- Walks AST trees to find and classify identifier references
