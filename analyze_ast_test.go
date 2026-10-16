package gocyclo_test

import (
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/fzipp/gocyclo"
)

// analyzeSource parses src as the file "p.go" and returns the "<complexity>
// <package> <function> <file:line:column>" lines for its functions, in source
// order.
func analyzeSource(t *testing.T, src string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	stats := gocyclo.AnalyzeASTFile(f, fset, nil)
	lines := make([]string, len(stats))
	for i, s := range stats {
		lines[i] = s.String()
	}
	return lines
}

func checkLines(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestAnalyzeASTFileValueSpecNames(t *testing.T) {
	got := analyzeSource(t, `package p

var a, b = func() {}, func(x bool) {
	if x {
	}
}

var c, d, e = 1, func() {}, "s"
`)
	checkLines(t, got, []string{
		"1 p a p.go:3:12",
		"2 p b p.go:3:23",
		"1 p d p.go:8:18",
	})
}

func TestAnalyzeASTFileReceivers(t *testing.T) {
	got := analyzeSource(t, `package p

type S struct{}

func (S) value()       {}
func (*S) pointer()    {}
func (s (S)) paren()   {}
func (s *(S)) parenP() {}
func (s (*S)) pparen() {}

type G[T any] struct{}

func (G[T]) generic()     {}
func (*G[T]) genericP()   {}
func (g (G[T])) genericR() {}

type M[K comparable, V any] struct{}

func (M[K, V]) multi()   {}
func (*M[K, V]) multiP() {}
`)
	checkLines(t, got, []string{
		"1 p (S).value p.go:5:1",
		"1 p (*S).pointer p.go:6:1",
		"1 p (S).paren p.go:7:1",
		"1 p (*S).parenP p.go:8:1",
		"1 p (*S).pparen p.go:9:1",
		"1 p (G).generic p.go:13:1",
		"1 p (*G).genericP p.go:14:1",
		"1 p (G).genericR p.go:15:1",
		"1 p (M).multi p.go:19:1",
		"1 p (*M).multiP p.go:20:1",
	})
}

func TestAnalyzeASTFileIgnoreDirectives(t *testing.T) {
	got := analyzeSource(t, `package p

//gocyclo:ignore
func ignoredFunc() {}

//gocyclo:ignore
var ignoredVar = func() {}

//gocyclo:ignore
var (
	ignoredByGroup1 = func() {}
	ignoredByGroup2 = func() {}
)

var (
	//gocyclo:ignore
	ignoredBySpec = func() {}
	keptInGroup   = func() {}
)

var (
	//gocyclo:ignore
	ignoredPair1, ignoredPair2 = func() {}, func() {}
)

// gocyclo:ignore
func keptSpaceBeforeDirective() {}

//gocyclo:unknown
func keptUnknownDirective() {}
`)
	checkLines(t, got, []string{
		"1 p keptInGroup p.go:18:18",
		"1 p keptSpaceBeforeDirective p.go:27:1",
		"1 p keptUnknownDirective p.go:30:1",
	})
}

func TestAnalyzeASTFileAppendsToStats(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", "package p\n\nfunc f() {}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	existing := gocyclo.Stats{{PkgName: "q", FuncName: "g", Complexity: 7}}
	got := gocyclo.AnalyzeASTFile(f, fset, existing)
	want := gocyclo.Stats{
		{PkgName: "q", FuncName: "g", Complexity: 7},
		{PkgName: "p", FuncName: "f", Complexity: 1, Pos: token.Position{
			Filename: "p.go", Offset: 11, Line: 3, Column: 1,
		}},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAnalyzeASTFileComplexity(t *testing.T) {
	// Each function's complexity is the number in its name. Nested function
	// literals count towards the enclosing declaration.
	got := analyzeSource(t, `package p

func f1() {
	switch {
	default:
	}
	select {
	default:
	}
	_ = !true
}

func f6(a, b, c bool, ch chan int) {
	if a && (b || c) {
	}
	for range ch {
	}
	_ = func() {
		if a {
		}
	}
}

func f4(x any, ch chan int) {
	switch x.(type) {
	case int, string:
	case nil:
	default:
	}
	select {
	case <-ch:
	default:
	}
}
`)
	checkLines(t, got, []string{
		"1 p f1 p.go:3:1",
		"6 p f6 p.go:13:1",
		"4 p f4 p.go:24:1",
	})
}
