package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"sort"
	"strings"
)

// checkGeneratedPackage type-checks the generated .go files as one package and
// reports declaration conflicts.
//
// formatGo only parses: go/format accepts a file that declares the same
// identifier twice, so a naming collision between two model elements used to
// pass generation and fail hours later at `go build` with "redeclared in this
// block", pointing at generated code instead of at the model. Parsing is
// per-file anyway, and package scope is exactly what a collision lives in — so
// this check runs once over the whole file set.
//
// Imports are deliberately stubbed rather than resolved: the generated package
// names a module that does not exist yet on disk (its own go.mod is part of the
// same output), so no importer can resolve it. Everything that depends on an
// import is therefore unknowable here, and only the errors that are decidable
// from the files alone are reported — see keepError.
//
// Files are grouped by directory: a generation can emit more than one package
// (the GraphQL resolver lives in graph/), and type-checking them together would
// report every cross-package reference as undefined.
func checkGeneratedPackage(files []GeneratedFile) error {
	byDir := map[string][]GeneratedFile{}
	var dirs []string
	for _, f := range files {
		if !strings.HasSuffix(f.Name, ".go") {
			continue
		}
		dir := path.Dir(f.Name)
		if _, seen := byDir[dir]; !seen {
			dirs = append(dirs, dir)
		}
		byDir[dir] = append(byDir[dir], f)
	}
	for _, dir := range dirs {
		if err := checkOnePackage(byDir[dir]); err != nil {
			return err
		}
	}
	return nil
}

func checkOnePackage(files []GeneratedFile) error {
	fset := token.NewFileSet()
	var parsed []*ast.File
	for _, f := range files {
		file, err := parser.ParseFile(fset, f.Name, f.Content, parser.SkipObjectResolution)
		if err != nil {
			// formatGo already rejected unparseable output; reaching here means
			// a file bypassed it.
			return fmt.Errorf("generated %s is not valid Go: %w", f.Name, err)
		}
		parsed = append(parsed, file)
	}
	if len(parsed) == 0 {
		return nil
	}

	var problems []string
	conf := types.Config{
		Importer:                 stubImporter{},
		DisableUnusedImportCheck: true,
		Error: func(err error) {
			if msg := keepError(err); msg != "" {
				problems = append(problems, msg)
			}
		},
	}
	// The returned error is the first collected one; Error above sees them all,
	// so it is ignored in favour of the filtered set.
	_, _ = conf.Check(parsed[0].Name.Name, fset, parsed, nil)

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("generated package does not compile:\n\t%s", strings.Join(problems, "\n\t"))
}

// keepError decides whether a type-checker diagnostic is trustworthy given that
// imports are stubbed. A redeclaration is decided entirely by the package's own
// declarations, so it is always real; anything else may be a consequence of an
// import that could not be resolved ("undefined: eventstore.Store") and would be
// a false alarm.
func keepError(err error) string {
	terr, ok := err.(types.Error)
	if !ok {
		return ""
	}
	if !strings.Contains(terr.Msg, "redeclared") {
		return ""
	}
	return fmt.Sprintf("%s: %s", terr.Fset.Position(terr.Pos), terr.Msg)
}

// stubImporter resolves every import path to an empty package, so type-checking
// gets past the import declarations without needing the module on disk.
type stubImporter struct{}

func (stubImporter) Import(path string) (*types.Package, error) {
	name := path
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	pkg := types.NewPackage(path, name)
	pkg.MarkComplete()
	return pkg, nil
}
