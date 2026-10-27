// Copyright 2020 Frederik Zipp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gocyclo

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Analyze calculates the cyclomatic complexities of the functions and methods
// in the Go source code files in the given paths. If a path is a directory
// all Go files under that directory are analyzed recursively.
// Files with paths matching the 'ignore' regular expressions are skipped.
// The 'ignore' parameter can be nil, meaning that no files are skipped.
// A path that does not exist, a directory that cannot be read, or a file
// that cannot be parsed is logged and skipped, and the analysis continues
// with the remaining files. Use AnalyzePaths to receive these problems as
// an error instead.
func Analyze(paths []string, ignore *regexp.Regexp) Stats {
	a := pathAnalyzer{ignore: ignore}
	a.analyzePaths(paths)
	for _, err := range a.errs {
		log.Println(err)
	}
	return a.stats
}

// AnalyzePaths is like Analyze, but instead of logging the problems it
// encounters it returns them, joined with errors.Join, after analyzing
// every file it could. The returned Stats are valid even if err is non-nil.
func AnalyzePaths(paths []string, ignore *regexp.Regexp) (Stats, error) {
	a := pathAnalyzer{ignore: ignore}
	a.analyzePaths(paths)
	return a.stats, errors.Join(a.errs...)
}

type pathAnalyzer struct {
	ignore *regexp.Regexp
	stats  Stats
	errs   []error
}

func (a *pathAnalyzer) analyzePaths(paths []string) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			a.errs = append(a.errs, fmt.Errorf("could not get file info for path %q: %w", path, err))
			continue
		}
		if info.IsDir() {
			a.analyzeDir(path)
		} else {
			a.analyzeFile(path)
		}
	}
}

func (a *pathAnalyzer) analyzeDir(dirname string) {
	// The callback never returns an error, so neither does WalkDir.
	_ = filepath.WalkDir(dirname, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Record the entry that could not be read and continue with
			// its siblings rather than abandoning the rest of the walk.
			a.errs = append(a.errs, err)
			return nil
		}

		if entry.IsDir() {
			// A directory named explicitly by the caller is always analyzed.
			if path != dirname && skipDir(entry.Name()) {
				return filepath.SkipDir
			}
		} else if isGoFile(entry.Name()) {
			a.analyzeFile(path)
		}
		return nil
	})
}

func skipDir(name string) bool {
	switch name {
	case "testdata", "vendor":
		return true
	}

	return strings.HasPrefix(name, ".") && name != "." && name != ".." ||
		strings.HasPrefix(name, "_")
}

func isGoFile(filename string) bool {
	return strings.HasSuffix(filename, ".go")
}

func (a *pathAnalyzer) analyzeFile(path string) {
	if isIgnored(path, a.ignore) {
		return
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		a.errs = append(a.errs, err)
		return
	}
	a.stats = AnalyzeASTFile(f, fset, a.stats)
}

func isIgnored(path string, ignore *regexp.Regexp) bool {
	return ignore != nil && ignore.MatchString(path)
}

// AnalyzeASTFile calculates the cyclomatic complexities of the functions
// and methods in the abstract syntax tree (AST) of a parsed Go file and
// appends the results to the given Stats slice.
func AnalyzeASTFile(f *ast.File, fs *token.FileSet, s Stats) Stats {
	analyzer := &fileAnalyzer{
		file:    f,
		fileSet: fs,
		stats:   s,
	}
	return analyzer.analyze()
}

type fileAnalyzer struct {
	file    *ast.File
	fileSet *token.FileSet
	stats   Stats
}

func (a *fileAnalyzer) analyze() Stats {
	for _, decl := range a.file.Decls {
		a.analyzeDecl(decl)
	}
	return a.stats
}

func (a *fileAnalyzer) analyzeDecl(d ast.Decl) {
	switch decl := d.(type) {
	case *ast.FuncDecl:
		a.addStatIfNotIgnored(decl, funcName(decl), decl.Doc)
	case *ast.GenDecl:
		for _, spec := range decl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if parseDirectives(valueSpec.Doc).HasIgnore() {
				continue
			}
			for i, value := range valueSpec.Values {
				funcLit, ok := value.(*ast.FuncLit)
				if !ok || i >= len(valueSpec.Names) {
					continue
				}
				a.addStatIfNotIgnored(funcLit, valueSpec.Names[i].Name, decl.Doc)
			}
		}
	}
}

func (a *fileAnalyzer) addStatIfNotIgnored(node ast.Node, funcName string, doc *ast.CommentGroup) {
	if parseDirectives(doc).HasIgnore() {
		return
	}
	a.stats = append(a.stats, Stat{
		PkgName:    a.file.Name.Name,
		FuncName:   funcName,
		Complexity: Complexity(node),
		Pos:        a.fileSet.Position(node.Pos()),
	})
}

// funcName returns the name representation of a function or method:
// "(Type).Name" for methods or simply "Name" for functions.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && fn.Recv.NumFields() > 0 {
		typ := fn.Recv.List[0].Type
		return fmt.Sprintf("(%s).%s", recvString(typ), fn.Name)
	}
	return fn.Name.Name
}
