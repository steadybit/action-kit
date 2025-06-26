// Copyright 2025 steadybit GmbH. All rights reserved.
package utils

import (
	"os"
	"os/exec"
)

func LocateExecutable(name, envVar string) string {
	if envVar != "" {
		if fromEnv := os.Getenv(envVar); fromEnv != "" {
			return fromEnv
		}
	}

	if fromLookPath, err := exec.LookPath(name); err == nil {
		return fromLookPath
	}

	return name
}
