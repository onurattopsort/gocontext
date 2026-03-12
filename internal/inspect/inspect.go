// Package inspect provides AST extraction utilities for Go source code.
// It wraps go/parser, go/ast, go/token, and go/doc to provide progressive
// disclosure of a Go codebase's structure.
package inspect

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PackageSummary holds a package's import path and its synopsis.
type PackageSummary struct {
	ImportPath string `json:"import_path"`
	Synopsis   string `json:"synopsis,omitempty"`
}

// shouldSkipDir returns true for directories that should be skipped during walks.
func shouldSkipDir(name string) bool {
	switch name {
	case "vendor", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// Tree walks dir recursively, parsing package-level doc comments.
// It returns a list of PackageSummary sorted by import path.
func Tree(dir string) ([]PackageSummary, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	modulePath, moduleRoot, err := findModulePath(dir)
	if err != nil {
		modulePath = ""
		moduleRoot = dir
	}

	var results []PackageSummary

	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip inaccessible dirs
		}
		if !info.IsDir() {
			return nil
		}
		if path != dir && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}

		fset := token.NewFileSet()
		pkgs, parseErr := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)
		if parseErr != nil || len(pkgs) == 0 {
			return nil
		}

		for _, pkg := range pkgs {
			d := doc.New(pkg, "", doc.AllDecls)
			synopsis := doc.Synopsis(d.Doc)

			importPath := computeImportPath(path, moduleRoot, modulePath)

			results = append(results, PackageSummary{
				ImportPath: importPath,
				Synopsis:   synopsis,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ImportPath < results[j].ImportPath
	})
	return results, nil
}

// computeImportPath derives an import path from a filesystem path
// using the module root and module path.
func computeImportPath(dir, moduleRoot, modulePath string) string {
	rel, err := filepath.Rel(moduleRoot, dir)
	if err != nil || rel == "." {
		if modulePath != "" {
			return modulePath
		}
		return filepath.Base(dir)
	}
	rel = filepath.ToSlash(rel)
	if modulePath != "" {
		return modulePath + "/" + rel
	}
	return rel
}

// findModulePath reads go.mod in or above dir to find the module path.
func findModulePath(dir string) (modulePath, moduleRoot string, err error) {
	cur := dir
	for {
		gomod := filepath.Join(cur, "go.mod")
		data, readErr := os.ReadFile(gomod)
		if readErr == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module")), cur, nil
				}
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", "", fmt.Errorf("go.mod not found above %s", dir)
}

// PackageDetail holds full extracted documentation for a package.
type PackageDetail struct {
	Name       string       `json:"name"`
	ImportPath string       `json:"import_path"`
	Doc        string       `json:"doc,omitempty"`
	Funcs      []FuncDetail `json:"funcs,omitempty"`
	Types      []TypeDetail `json:"types,omitempty"`
}

// FuncDetail holds a function signature and its doc.
type FuncDetail struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Doc       string `json:"doc,omitempty"`
}

// TypeDetail holds information about an exported type.
type TypeDetail struct {
	Name    string       `json:"name"`
	Kind    string       `json:"kind"` // "struct", "interface", or "type"
	Doc     string       `json:"doc,omitempty"`
	Methods []FuncDetail `json:"methods,omitempty"`
	Funcs   []FuncDetail `json:"funcs,omitempty"` // associated constructors
}

// Package parses the package at the given directory path and returns
// exported symbols with documentation.
func Package(dir string) (*PackageDetail, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	modulePath, moduleRoot, modErr := findModulePath(dir)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing package at %s: %w", dir, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no Go packages found in %s", dir)
	}

	// Pick the non-test package (there should typically be one).
	var pkg *ast.Package
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			pkg = p
			break
		}
	}
	if pkg == nil {
		for _, p := range pkgs {
			pkg = p
			break
		}
	}

	d := doc.New(pkg, "", doc.AllDecls)

	importPath := dir
	if modErr == nil {
		importPath = computeImportPath(dir, moduleRoot, modulePath)
	}

	detail := &PackageDetail{
		Name:       d.Name,
		ImportPath: importPath,
		Doc:        strings.TrimSpace(d.Doc),
	}

	// Top-level functions (exported only).
	for _, f := range d.Funcs {
		if !ast.IsExported(f.Name) {
			continue
		}
		detail.Funcs = append(detail.Funcs, FuncDetail{
			Name:      f.Name,
			Signature: formatFuncDecl(fset, f.Decl),
			Doc:       strings.TrimSpace(f.Doc),
		})
	}

	// Types.
	for _, t := range d.Types {
		if !ast.IsExported(t.Name) {
			continue
		}
		td := TypeDetail{
			Name: t.Name,
			Kind: typeKind(t),
			Doc:  strings.TrimSpace(t.Doc),
		}
		for _, m := range t.Methods {
			if !ast.IsExported(m.Name) {
				continue
			}
			td.Methods = append(td.Methods, FuncDetail{
				Name:      m.Name,
				Signature: formatFuncDecl(fset, m.Decl),
				Doc:       strings.TrimSpace(m.Doc),
			})
		}
		for _, f := range t.Funcs {
			if !ast.IsExported(f.Name) {
				continue
			}
			td.Funcs = append(td.Funcs, FuncDetail{
				Name:      f.Name,
				Signature: formatFuncDecl(fset, f.Decl),
				Doc:       strings.TrimSpace(f.Doc),
			})
		}
		detail.Types = append(detail.Types, td)
	}

	return detail, nil
}

// typeKind returns "struct", "interface", or "type" for a doc.Type.
func typeKind(t *doc.Type) string {
	if t.Decl == nil || len(t.Decl.Specs) == 0 {
		return "type"
	}
	ts, ok := t.Decl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return "type"
	}
	switch ts.Type.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	default:
		return "type"
	}
}

// formatFuncDecl renders a function declaration without its body.
func formatFuncDecl(fset *token.FileSet, decl *ast.FuncDecl) string {
	// Clone the decl without body.
	clone := *decl
	clone.Body = nil
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, &clone)
	return buf.String()
}

// Symbol locates a specific exported type/const/var in the package at dir
// and returns its Go source definition.
func Symbol(dir, name string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing package: %w", err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.Name == name {
								return renderNode(fset, d)
							}
						case *ast.ValueSpec:
							for _, ident := range s.Names {
								if ident.Name == name {
									return renderNode(fset, d)
								}
							}
						}
					}
				case *ast.FuncDecl:
					if d.Recv == nil && d.Name.Name == name {
						return renderNode(fset, d)
					}
				}
			}
		}
	}

	return "", fmt.Errorf("symbol %q not found in package at %s", name, dir)
}

// Body locates a function or method and returns its full source code.
// For methods, name should be "ReceiverType.MethodName".
func Body(dir, name string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	receiverType, funcName, isMethod := parseMethodName(name)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parsing package: %w", err)
	}

	for _, pkg := range pkgs {
		for filePath, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if isMethod {
					if fn.Recv == nil || fn.Name.Name != funcName {
						continue
					}
					if !matchesReceiver(fn, receiverType) {
						continue
					}
				} else {
					if fn.Name.Name != funcName {
						continue
					}
				}

				// Read the raw source and extract the exact byte range.
				src, readErr := os.ReadFile(filePath)
				if readErr != nil {
					return "", fmt.Errorf("reading source file %s: %w", filePath, readErr)
				}

				start := fset.Position(fn.Pos())
				end := fset.Position(fn.End())
				body := string(src[start.Offset:end.Offset])

				return body, nil
			}
		}
	}

	return "", fmt.Errorf("function/method %q not found in package at %s", name, dir)
}

// parseMethodName splits "Receiver.Method" into parts.
func parseMethodName(name string) (receiver, method string, isMethod bool) {
	if idx := strings.LastIndex(name, "."); idx > 0 {
		return name[:idx], name[idx+1:], true
	}
	return "", name, false
}

// matchesReceiver checks if a FuncDecl's receiver matches the given type name,
// accounting for pointer receivers.
func matchesReceiver(fn *ast.FuncDecl, typeName string) bool {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return false
	}
	recvType := fn.Recv.List[0].Type
	// Unwrap pointer.
	if star, ok := recvType.(*ast.StarExpr); ok {
		recvType = star.X
	}
	if ident, ok := recvType.(*ast.Ident); ok {
		return ident.Name == typeName
	}
	// Handle indexed expressions for generic receivers.
	if idx, ok := recvType.(*ast.IndexExpr); ok {
		if ident, ok := idx.X.(*ast.Ident); ok {
			return ident.Name == typeName
		}
	}
	return false
}

// renderNode formats an AST node back to Go source code.
func renderNode(fset *token.FileSet, node ast.Node) (string, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return "", fmt.Errorf("formatting node: %w", err)
	}
	return buf.String(), nil
}

// Reference represents a single usage of a symbol in the codebase.
type Reference struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Context string `json:"context"` // the source line containing the reference
	Kind    string `json:"kind"`    // "call", "type", "field", "assign", "other"
}

// Refs finds all references to a symbol across the directory tree.
// It walks all Go files (skipping vendor/hidden/test), parses ASTs,
// and looks for identifiers matching the given name.
func Refs(rootDir, symbolName string) ([]Reference, error) {
	rootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	var results []Reference

	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if path != rootDir && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		lines := strings.Split(string(src), "\n")

		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok || ident.Name != symbolName {
				return true
			}

			pos := fset.Position(ident.Pos())
			relPath, relErr := filepath.Rel(rootDir, pos.Filename)
			if relErr != nil {
				relPath = pos.Filename
			}

			context := ""
			if pos.Line > 0 && pos.Line <= len(lines) {
				context = strings.TrimSpace(lines[pos.Line-1])
			}

			kind := classifyRef(ident, file)

			results = append(results, Reference{
				File:    relPath,
				Line:    pos.Line,
				Column:  pos.Column,
				Context: context,
				Kind:    kind,
			})
			return true
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking for refs: %w", err)
	}

	return results, nil
}

// classifyRef attempts to classify how an identifier is used.
func classifyRef(ident *ast.Ident, file *ast.File) string {
	// Walk the file to find the parent node of this ident.
	var kind string
	ast.Inspect(file, func(n ast.Node) bool {
		if kind != "" {
			return false
		}
		switch parent := n.(type) {
		case *ast.CallExpr:
			if fun, ok := parent.Fun.(*ast.Ident); ok && fun == ident {
				kind = "call"
				return false
			}
			if sel, ok := parent.Fun.(*ast.SelectorExpr); ok && sel.Sel == ident {
				kind = "call"
				return false
			}
		case *ast.SelectorExpr:
			if parent.Sel == ident {
				// Could be field access or method call — parent check already handles call.
				kind = "field"
				return false
			}
		case *ast.TypeSpec:
			if parent.Name == ident {
				kind = "type"
				return false
			}
		case *ast.AssignStmt:
			for _, lhs := range parent.Lhs {
				if lhs == ident {
					kind = "assign"
					return false
				}
			}
		case *ast.FuncDecl:
			if parent.Name == ident {
				kind = "decl"
				return false
			}
		case *ast.Field:
			for _, name := range parent.Names {
				if name == ident {
					kind = "field"
					return false
				}
			}
		}
		return true
	})
	if kind == "" {
		kind = "ref"
	}
	return kind
}

// OverviewPackage holds summary info for a single package in the overview.
type OverviewPackage struct {
	ImportPath string   `json:"import_path"`
	Synopsis   string   `json:"synopsis,omitempty"`
	Dir        string   `json:"dir"`
	Types      []string `json:"types,omitempty"`
	Funcs      []string `json:"funcs,omitempty"`
}

// Overview produces a single-call summary of an entire module: every package
// with its exported type names and function signatures.
func Overview(dir string) ([]OverviewPackage, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	modulePath, moduleRoot, err := findModulePath(dir)
	if err != nil {
		modulePath = ""
		moduleRoot = dir
	}

	var results []OverviewPackage

	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if path != dir && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}

		fset := token.NewFileSet()
		pkgs, parseErr := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ParseComments)
		if parseErr != nil || len(pkgs) == 0 {
			return nil
		}

		for _, pkg := range pkgs {
			if strings.HasSuffix(pkg.Name, "_test") {
				continue
			}

			d := doc.New(pkg, "", doc.AllDecls)
			importPath := computeImportPath(path, moduleRoot, modulePath)
			relDir, _ := filepath.Rel(moduleRoot, path)
			if relDir == "" || relDir == "." {
				relDir = "."
			}

			op := OverviewPackage{
				ImportPath: importPath,
				Synopsis:   doc.Synopsis(d.Doc),
				Dir:        relDir,
			}

			for _, t := range d.Types {
				if !ast.IsExported(t.Name) {
					continue
				}
				op.Types = append(op.Types, t.Name)
			}

			for _, f := range d.Funcs {
				if !ast.IsExported(f.Name) {
					continue
				}
				op.Funcs = append(op.Funcs, formatFuncDecl(fset, f.Decl))
			}

			// Also include methods as "Type.Method" signatures.
			for _, t := range d.Types {
				if !ast.IsExported(t.Name) {
					continue
				}
				for _, f := range t.Funcs {
					if !ast.IsExported(f.Name) {
						continue
					}
					op.Funcs = append(op.Funcs, formatFuncDecl(fset, f.Decl))
				}
				for _, m := range t.Methods {
					if !ast.IsExported(m.Name) {
						continue
					}
					op.Funcs = append(op.Funcs, formatFuncDecl(fset, m.Decl))
				}
			}

			results = append(results, op)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ImportPath < results[j].ImportPath
	})
	return results, nil
}

// ResolveImportPath takes either a filesystem path or an import path
// and returns the absolute filesystem path to the package directory.
// It searches relative to the module root.
func ResolveImportPath(input string) (string, error) {
	// If it's already an existing directory, use it directly.
	abs, err := filepath.Abs(input)
	if err == nil {
		if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
			return abs, nil
		}
	}

	// Try to resolve as a module-relative import path.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	modulePath, moduleRoot, err := findModulePath(cwd)
	if err != nil {
		return "", fmt.Errorf("finding module root: %w", err)
	}

	// Strip module prefix to get relative path.
	if strings.HasPrefix(input, modulePath) {
		rel := strings.TrimPrefix(input, modulePath)
		rel = strings.TrimPrefix(rel, "/")
		candidate := filepath.Join(moduleRoot, rel)
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
	}

	// Try as a path relative to module root.
	candidate := filepath.Join(moduleRoot, input)
	if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
		return candidate, nil
	}

	return "", fmt.Errorf("cannot resolve %q to a package directory (module: %s, root: %s)", input, modulePath, moduleRoot)
}
