// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH
//go:build windows

package action_kit_sdk

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/steadybit/extension-kit/extsignals"
)

func TestWindowsSignalsWithExternalProcess(t *testing.T) {
	const source = `
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM) 

	select {
	case s := <-c:
		if s == os.Interrupt {
			fmt.Println("SUCCESS: Received os.Interrupt")
			os.Exit(0) 
		} else {
			log.Fatalf("FAIL: Wrong signal received: got %q, want %q\n", s, os.Interrupt)
		}
	case <-time.After(5 * time.Second): 
		log.Fatalf("FAIL: Timeout waiting for Ctrl+Break\n")
	}
}
`
	tmpDir := t.TempDir()
	t.Logf("Using temp directory: %s", tmpDir)

	baseName := "main"
	srcPath := filepath.Join(tmpDir, baseName+".go")
	err := os.WriteFile(srcPath, []byte(source), 0644)
	if err != nil {
		t.Fatalf("Failed to write source file %v: %v", srcPath, err)
	}
	t.Logf("Source file written: %s", srcPath)

	exePath := filepath.Join(tmpDir, baseName+".exe")
	buildCmd := exec.Command("go", "build", "-o", exePath, srcPath)
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdout = os.Stdout
	buildCmd.Dir = tmpDir
	t.Logf("Compiling: %s", buildCmd.String())
	err = buildCmd.Run()
	if err != nil {
		t.Fatalf("Failed to compile %v: %v", srcPath, err)
	}
	if _, err := os.Stat(exePath); err != nil {
		t.Fatalf("Compiled executable not found at %s: %v", exePath, err)
	}
	t.Logf("Compiled executable: %s", exePath)
	cmd := exec.Command(exePath)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	t.Logf("Starting command: %s", cmd.String())
	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start command %s: %v", exePath, err)
	}
	pid := cmd.Process.Pid
	t.Logf("Process started with PID: %d. Waiting a moment before sending signal...", pid)

	time.Sleep(500 * time.Millisecond)

	t.Logf("Sending CTRL_BREAK_EVENT signal via Kill(%d)", pid)
	err = extsignals.Kill(pid)
	if err != nil {
		_ = cmd.Process.Kill()
		waitErr := cmd.Wait()
		t.Logf("Kill signal send failed. Process wait result after kill attempt: %v", waitErr)
		t.Logf("Stdout: %s", stdoutBuf.String())
		t.Logf("Stderr: %s", stderrBuf.String())
		t.Fatalf("Failed to send signal using Kill(%d): %v", pid, err)
	}
	t.Logf("Signal sent via Kill(%d). Waiting for process to exit...", pid)

	err = cmd.Wait()

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()
	t.Logf("Process exited. Wait error: %v", err)
	t.Logf("Process Stdout:\n%s", stdoutStr)
	t.Logf("Process Stderr:\n%s", stderrStr)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if strings.Contains(stderrStr, "FAIL: Timeout") {
				t.Errorf("Test failed: Target process timed out waiting for signal.")
			} else if strings.Contains(stderrStr, "FAIL: Wrong signal") {
				t.Errorf("Test failed: Target process received the wrong signal.")
			} else {
				t.Errorf("Test failed: Target process exited with code %d. Stderr:\n%s", exitErr.ExitCode(), stderrStr)
			}
		} else {
			t.Errorf("Test failed: Error waiting for target process: %v", err)
		}
	} else {
		if !strings.Contains(stdoutStr, "SUCCESS: Received os.Interrupt") {
			t.Errorf("Test potentially failed: Process exited successfully, but expected SUCCESS message not found in stdout.")
		} else {
			t.Logf("Test passed: Target process received os.Interrupt and exited successfully.")
		}
	}
}
