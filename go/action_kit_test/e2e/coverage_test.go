package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCovered builds testdata/covprog with -cover once, runs it with arg and returns its coverage
// meta-data and counters, the same files an extension serves on /coverage/meta and /coverage/counters.
func runCovered(t *testing.T, binary, arg string) ([]byte, []byte) {
	dir := t.TempDir()
	cmd := exec.Command(binary, arg)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+dir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	read := func(pattern string) []byte {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		require.NoError(t, err)
		require.Len(t, matches, 1, pattern)
		data, err := os.ReadFile(matches[0])
		require.NoError(t, err)
		return data
	}
	return read("covmeta.*"), read("covcounters.*")
}

func TestCoverage_accumulates_runs(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "covprog")
	out, err := exec.Command("go", "build", "-cover", "-o", binary, "./testdata/covprog").CombinedOutput()
	require.NoError(t, err, string(out))

	dir := filepath.Join(t.TempDir(), "covdata")
	report := filepath.Join(t.TempDir(), "report.out")

	meta, counters := runCovered(t, binary, "first")
	require.NoError(t, addCoverageRun(dir, meta, counters))
	require.NoError(t, writeCoverageReport(dir, report))
	assert.Equal(t, map[string]bool{"first": true, "second": false}, coveredBranches(t, report))

	meta, counters = runCovered(t, binary, "second")
	require.NoError(t, addCoverageRun(dir, meta, counters))
	require.NoError(t, writeCoverageReport(dir, report))
	assert.Equal(t, map[string]bool{"first": true, "second": true}, coveredBranches(t, report), "the second run must add to the report, not replace it")
}

// coveredBranches reads the text report and tells, per branch of covprog, whether it was executed.
// The first branch is the block starting on line 10, the second the one starting on line 12.
func coveredBranches(t *testing.T, report string) map[string]bool {
	data, err := os.ReadFile(report)
	require.NoError(t, err)
	covered := map[string]bool{"first": false, "second": false}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "covprog/main.go:") {
			continue
		}
		fields := strings.Fields(line)
		if fields[len(fields)-1] == "0" {
			continue
		}
		switch {
		case strings.Contains(line, "main.go:10."):
			covered["first"] = true
		case strings.Contains(line, "main.go:12."):
			covered["second"] = true
		}
	}
	return covered
}
