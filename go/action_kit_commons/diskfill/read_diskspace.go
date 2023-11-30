package diskfill

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type DiskUsage struct {
	Capacity  int64
	Used      int64
	Available int64
}

func readDiskUsage(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) (*DiskUsage, error) {
	bundle, err := createBundle(ctx, r, sidecar, opts, "df", "--sync", "-k", "--output=source,target,fstype,file,size,avail,used", mountpointInContainer)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := bundle.Remove(); err != nil {
			log.Warn().Str("id", bundle.ContainerId()).Err(err).Msg("failed to remove bundle")
		}
	}()

	var outb, errb bytes.Buffer
	err = r.Run(ctx, bundle, runc.IoOpts{Stdout: &outb, Stderr: &errb})
	defer func() {
		if err := r.Delete(context.Background(), bundle.ContainerId(), true); err != nil {
			log.Warn().Str("id", bundle.ContainerId()).Err(err).Msg("failed to delete container")
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w: %s", err, errb.String())
	}

	usage, err := CalculateDiskUsage(bytes.NewReader(outb.Bytes()))
	if err != nil {
		log.Warn().Err(err).Msg("failed to calculate disk usage")
		return nil, err
	}
	log.Trace().Msgf("Disk usage: %v", usage)
	return &usage, nil
}

var DfHeader = []string{
	"Filesystem",

	// Mounted on
	"Mounted",
	"on",

	"Type",
	"File",
	"1K-blocks",
	"Avail",
	"Used",
}

func CalculateDiskUsage(r io.Reader) (DiskUsage, error) {
	headerFound := false
	scanner := bufio.NewScanner(r)
	var result DiskUsage

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		log.Debug().Msgf("line: %q, fields: %v", line, fields)
		if !headerFound {
			if fields[0] == DfHeader[0] {
				if !reflect.DeepEqual(fields, DfHeader) {
					return DiskUsage{}, fmt.Errorf("unexpected 'df' command header order (%v, expected %v, output: %q)", fields, DfHeader, line)
				}
				headerFound = true
				continue
			}
		} else {
			if len(fields) != 7 {
				return DiskUsage{}, fmt.Errorf("unexpected row column number %v (expected %v)", fields, 7)
			}
			if iv, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
				result.Capacity = iv
			} else {
				return DiskUsage{}, fmt.Errorf("parse error %w", err)
			}
			if iv, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
				result.Available = iv
			} else {
				return DiskUsage{}, fmt.Errorf("parse error %w", err)
			}
			if iv, err := strconv.ParseInt(fields[6], 10, 64); err == nil {
				result.Used = iv
			} else {
				return DiskUsage{}, fmt.Errorf("parse error %w", err)
			}
			log.Debug().Msgf("result: %+v", result)
			return result, nil
		}
	}

	return DiskUsage{}, errors.New("unexpected 'df' command output")
}
