package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// coverageDataDir keeps the raw coverage data of every extension run in this test process, outside
// the repository. A test binary can start several minikubes (one WithMinikube call per
// configuration); the text report is regenerated from all of them after each run, so a later run
// adds to the report instead of replacing it.
var coverageDataDir = sync.OnceValues(func() (string, error) { return os.MkdirTemp("", "e2e-covdata-") })

var (
	coverageRun int
	coverageMu  sync.Mutex
)

func processCoverage(extension *Extension, runtime Runtime) {
	if extension == nil || extension.Client == nil {
		log.Info().Msg("no extension to collect coverage from.")
		return
	}
	meta, err := extension.Client.R().Get("/coverage/meta")
	if err != nil || !meta.IsSuccess() {
		log.Info().Err(err).Msg("failed to get coverage meta. Did you compile with `-cover`? Did you add the coverage endpoints ('action_kit_sdk.RegisterCoverageEndpoints()')?")
		return
	}
	counters, err := extension.Client.R().Get("/coverage/counters")
	if err != nil || !counters.IsSuccess() {
		log.Info().Err(err).Msg("failed to get coverage counters. Did you compile with `-cover`? Did you add the coverage endpoints ('action_kit_sdk.RegisterCoverageEndpoints()')?")
		return
	}
	dir, err := coverageDataDir()
	if err != nil {
		log.Info().Err(err).Msg("failed to create coverage data directory.")
		return
	}
	if err := addCoverageRun(dir, meta.Body(), counters.Body()); err != nil {
		log.Info().Err(err).Msg("failed to store coverage data.")
		return
	}
	if err := writeCoverageReport(dir, fmt.Sprintf("e2e-coverage-%s.out", runtime)); err != nil {
		log.Info().Err(err).Msg("failed to convert coverage data.")
	}
}

// addCoverageRun stores one run's coverage data in dir, named the way `go tool covdata` pairs counters
// with their meta-data: covmeta.<hash> and covcounters.<hash>.<pid>.<nanotime>. The hash is taken from
// the meta-data, so runs of the same binary share it and runs of different builds stay apart.
func addCoverageRun(dir string, meta, counters []byte) error {
	coverageMu.Lock()
	defer coverageMu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	sum := sha256.Sum256(meta)
	hash := hex.EncodeToString(sum[:16])
	if err := os.WriteFile(filepath.Join(dir, "covmeta."+hash), meta, 0o644); err != nil {
		return err
	}
	coverageRun++
	name := fmt.Sprintf("covcounters.%s.%d.%d", hash, coverageRun, time.Now().UnixNano())
	return os.WriteFile(filepath.Join(dir, name), counters, 0o644)
}

// writeCoverageReport merges all runs in dir into one text report.
func writeCoverageReport(dir, report string) error {
	// The go toolchain on PATH is the one running this test binary, so PATH is already trusted here.
	out, err := exec.Command("go", "tool", "covdata", "textfmt", "-i", dir, "-o", report).CombinedOutput() //NOSONAR go:S4036
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
