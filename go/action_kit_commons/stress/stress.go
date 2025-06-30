// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package stress

import (
	"github.com/rs/zerolog/log"
	"strconv"
	"time"
)

type Opts struct {
	CpuWorkers   *int
	CpuLoad      int
	HddWorkers   *int
	HddBytes     string
	IoWorkers    *int
	IomixWorkers *int
	IomixBytes   string
	TempPath     string
	Timeout      time.Duration
	VmWorkers    *int
	VmHang       time.Duration
	VmBytes      string
}

type Stress interface {
	Exited() (bool, error)
	Start() error
	Stop()
}

func (o *Opts) Args() []string {
	args := []string{"--timeout", strconv.Itoa(int(o.Timeout.Seconds()))}
	if o.CpuWorkers != nil {
		args = append(args, "--cpu", strconv.Itoa(*o.CpuWorkers), "--cpu-load", strconv.Itoa(o.CpuLoad))
	}
	if o.HddWorkers != nil {
		args = append(args, "--hdd", strconv.Itoa(*o.HddWorkers))
	}
	if o.HddBytes != "" {
		args = append(args, "--hdd-bytes", o.HddBytes)
	}
	if o.IoWorkers != nil {
		args = append(args, "--io", strconv.Itoa(*o.IoWorkers))
	}
	if o.IomixWorkers != nil {
		args = append(args, "--iomix", strconv.Itoa(*o.IomixWorkers))
	}
	if o.IomixBytes != "" {
		args = append(args, "--iomix-bytes", strconv.Itoa(*o.IomixWorkers))
	}
	if o.TempPath != "" {
		args = append(args, "--temp-path", o.TempPath)
	}
	if o.VmWorkers != nil {
		args = append(args, "--vm", strconv.Itoa(*o.VmWorkers), "--vm-bytes", o.VmBytes, "--vm-hang", "0")
	}
	if log.Trace().Enabled() {
		args = append(args, "-v")
	}
	return args
}
