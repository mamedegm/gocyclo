package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// When this variable is set, the test binary runs main instead of the tests,
// so that the tests can check the output and exit code of the command.
const runMainEnv = "GOCYCLO_TEST_RUN_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		os.Args = append([]string{"gocyclo"}, os.Args[1:]...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runGocyclo runs the command with args and returns its standard output,
// standard error and exit code.
func runGocyclo(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), runMainEnv+"=1")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatal(err)
	}
	return outBuf.String(), errBuf.String(), code
}

const ifsFile = "../../testdata/ifs.go"

func TestCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdout    string
		code      int
		stderrSub string // substring expected on standard error
	}{
		{
			name: "all functions",
			args: []string{ifsFile},
			stdout: `3 testdata f3 ../../testdata/ifs.go:17:1
3 testdata f3nested ../../testdata/ifs.go:24:1
2 testdata f2 ../../testdata/ifs.go:6:1
2 testdata f2else ../../testdata/ifs.go:11:1
1 testdata f1 ../../testdata/ifs.go:3:1
`,
		},
		{
			name: "over with matches exits 1",
			args: []string{"-over", "2", ifsFile},
			stdout: `3 testdata f3 ../../testdata/ifs.go:17:1
3 testdata f3nested ../../testdata/ifs.go:24:1
`,
			code: 1,
		},
		{
			name: "over without matches exits 0",
			args: []string{"-over", "3", ifsFile},
		},
		{
			name: "top",
			args: []string{"-top", "2", ifsFile},
			stdout: `3 testdata f3 ../../testdata/ifs.go:17:1
3 testdata f3nested ../../testdata/ifs.go:24:1
`,
		},
		{
			// The average covers all analyzed functions, not just the shown ones.
			name: "average over all functions",
			args: []string{"-top", "1", "-avg", ifsFile},
			stdout: `3 testdata f3 ../../testdata/ifs.go:17:1
Average: 2.2
`,
		},
		{
			name:   "short average",
			args:   []string{"-top", "0", "-avg-short", ifsFile},
			stdout: "2.2\n",
		},
		{
			name:   "average without functions",
			args:   []string{"-avg", "-ignore", "ifs", ifsFile},
			stdout: "Average: 0\n",
		},
		{
			name: "ignore",
			args: []string{"-ignore", "ifs", ifsFile, "../../testdata/directory"},
			stdout: `1 directory a ../../testdata/directory/file1.go:3:1
1 directory b ../../testdata/directory/file2.go:3:1
`,
		},
		{
			name:      "missing path is reported and exits 2",
			args:      []string{"-top", "1", "does-not-exist.go", ifsFile},
			stdout:    "3 testdata f3 ../../testdata/ifs.go:17:1\n",
			code:      2,
			stderrSub: `gocyclo: could not get file info for path "does-not-exist.go"`,
		},
		{
			name:      "unparsable file is reported, the rest analyzed, exits 2",
			args:      []string{"../../testdata/broken"},
			stdout:    "1 broken ok ../../testdata/broken/ok.go:3:1\n",
			code:      2,
			stderrSub: "gocyclo: ../../testdata/broken/broken.go:3:",
		},
		{
			// Exit code 1 must mean only that -over found functions.
			name: "an error takes precedence over -over matches",
			args: []string{"-over", "2", ifsFile, "does-not-exist.go"},
			stdout: `3 testdata f3 ../../testdata/ifs.go:17:1
3 testdata f3nested ../../testdata/ifs.go:24:1
`,
			code:      2,
			stderrSub: `gocyclo: could not get file info for path "does-not-exist.go"`,
		},
		{
			name: "each error gets its own prefixed line",
			args: []string{"missing-a.go", "missing-b.go"},
			code: 2,
			stderrSub: `gocyclo: could not get file info for path "missing-a.go": stat missing-a.go: no such file or directory
gocyclo: could not get file info for path "missing-b.go"`,
		},
		{
			name:      "invalid ignore expression",
			args:      []string{"-ignore", "(", ifsFile},
			code:      2,
			stderrSub: "gocyclo: error parsing regexp",
		},
		{
			name:      "no paths prints usage",
			code:      2,
			stderrSub: "Usage:\n    gocyclo [flags] <Go file or directory> ...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runGocyclo(t, tt.args...)
			if stdout != tt.stdout {
				t.Errorf("standard output:\n%s\nwant:\n%s", stdout, tt.stdout)
			}
			if code != tt.code {
				t.Errorf("exit code: got %d, want %d", code, tt.code)
			}
			if tt.stderrSub == "" && stderr != "" {
				t.Errorf("unexpected standard error:\n%s", stderr)
			}
			if !strings.Contains(stderr, tt.stderrSub) {
				t.Errorf("standard error:\n%s\nwant it to contain:\n%s", stderr, tt.stderrSub)
			}
		})
	}
}
