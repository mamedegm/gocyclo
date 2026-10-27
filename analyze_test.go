package gocyclo_test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fzipp/gocyclo"
)

func funcNames(stats gocyclo.Stats) []string {
	names := make([]string, len(stats))
	for i, s := range stats {
		names[i] = s.FuncName
	}
	slices.Sort(names)
	return names
}

func TestAnalyzePathsContinuesAfterErrors(t *testing.T) {
	stats, err := gocyclo.AnalyzePaths(
		[]string{"does-not-exist.go", "testdata/broken", "testdata/ifs.go"}, nil)

	if got, want := funcNames(stats), []string{"f1", "f2", "f2else", "f3", "f3nested", "ok"}; !slices.Equal(got, want) {
		t.Errorf("analyzed functions %q, want %q", got, want)
	}
	if err == nil {
		t.Fatal("got no error, want one for the missing path and one for the unparsable file")
	}
	for _, want := range []string{`"does-not-exist.go"`, "testdata/broken/broken.go:3:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestAnalyzePathsContinuesAfterUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs Unix permissions that apply to the current user")
	}
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		src := "package " + name + "\n\nfunc " + name + "() {}\n"
		if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	unreadable := filepath.Join(root, "b")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })

	stats, err := gocyclo.AnalyzePaths([]string{root}, nil)

	// "c" comes after the unreadable "b" in the walk and must still be analyzed.
	if got, want := funcNames(stats), []string{"a", "c"}; !slices.Equal(got, want) {
		t.Errorf("analyzed functions %q, want %q", got, want)
	}
	if err == nil || !strings.Contains(err.Error(), unreadable) {
		t.Errorf("error %v does not name the unreadable directory %s", err, unreadable)
	}
}

func TestAnalyzeLogsErrorsAndContinues(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	stats := gocyclo.Analyze([]string{"testdata/broken"}, nil)

	if got, want := funcNames(stats), []string{"ok"}; !slices.Equal(got, want) {
		t.Errorf("analyzed functions %q, want %q", got, want)
	}
	if !strings.Contains(buf.String(), "testdata/broken/broken.go:3:") {
		t.Errorf("log output %q does not report the unparsable file", buf.String())
	}
}

func TestAnalyze(t *testing.T) {
	tests := []struct {
		paths []string
		want  string
	}{
		{
			[]string{"testdata/ifs.go"},
			`3 testdata f3 testdata/ifs.go:17:1
3 testdata f3nested testdata/ifs.go:24:1
2 testdata f2 testdata/ifs.go:6:1
2 testdata f2else testdata/ifs.go:11:1
1 testdata f1 testdata/ifs.go:3:1`,
		},
		{
			[]string{"testdata/loops.go"},
			`4 testdata l4 testdata/loops.go:20:1
3 testdata l3 testdata/loops.go:8:1
2 testdata l2 testdata/loops.go:3:1
2 testdata l2range testdata/loops.go:15:1`,
		},
		{
			[]string{"testdata/cases.go"},
			`3 testdata c3 testdata/cases.go:25:1
3 testdata c3default testdata/cases.go:32:1
3 testdata c3nested testdata/cases.go:40:1
2 testdata c2 testdata/cases.go:6:1
2 testdata c2default testdata/cases.go:12:1
2 testdata c2multi testdata/cases.go:19:1
1 testdata c1 testdata/cases.go:3:1`,
		},
		{
			[]string{"testdata/comms.go"},
			`3 testdata comm3 testdata/comms.go:18:1
3 testdata comm3default testdata/comms.go:25:1
3 testdata comm3nested testdata/comms.go:33:1
2 testdata comm2 testdata/comms.go:5:1
2 testdata comm2default testdata/comms.go:11:1`,
		},
		{
			[]string{"testdata/methods.go"},
			`2 testdata (S).m2 testdata/methods.go:8:1
2 testdata (*S).m2ptr testdata/methods.go:16:1
1 testdata (S).m1 testdata/methods.go:5:1
1 testdata (*S).m1ptr testdata/methods.go:13:1`,
		},
		{
			[]string{"testdata/literals.go"},
			`3 testdata lit3 testdata/literals.go:13:12
2 testdata lit2 testdata/literals.go:8:12
1 testdata lit1 testdata/literals.go:5:12`,
		},
		{
			[]string{"testdata/ignores.go"},
			`1 testdata notIgnoredUnknownDirective testdata/ignores.go:10:1
1 testdata notIgnoredNotADirective testdata/ignores.go:13:1`,
		},
		{
			[]string{"testdata/operators.go"},
			`3 testdata op3mixed testdata/operators.go:11:1
2 testdata op2or testdata/operators.go:3:1
2 testdata op2and testdata/operators.go:7:1`,
		},
		{
			[]string{"testdata/directory"},
			`1 directory a testdata/directory/file1.go:3:1
1 directory b testdata/directory/file2.go:3:1`,
		},
		{
			[]string{"testdata/ignore"},
			"",
		},
		// A directory named explicitly is analyzed even when its own name
		// would cause it to be skipped during a recursive walk.
		{
			[]string{"testdata/ignore/vendor"},
			"1 vendor a testdata/ignore/vendor/file.go:3:1",
		},
		{
			[]string{"testdata/ignore/testdata"},
			"1 testdata a testdata/ignore/testdata/file.go:3:1",
		},
		{
			[]string{"testdata/ignore/_underscore"},
			"1 underscore a testdata/ignore/_underscore/file.go:3:1",
		},
		{
			[]string{"testdata/ignore/.dotname"},
			"1 dotname a testdata/ignore/.dotname/file.go:3:1",
		},
	}

	for _, tt := range tests {
		stats := gocyclo.Analyze(tt.paths, nil).
			SortAndFilter(-1, 0)
		statLines := make([]string, len(stats))
		for i, s := range stats {
			statLines[i] = s.String()
		}
		got := strings.Join(statLines, "\n")
		if got != tt.want {
			t.Errorf("Analyzed %q and got:\n%s\n\twant:\n%s", tt.paths, got, tt.want)
		}
	}
}
