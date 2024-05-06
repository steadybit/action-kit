package diskfill

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_commons/runc"
	"io"
	"strconv"
	"strings"
)

type DiskUsage struct {
	Capacity  int64
	Used      int64
	Available int64
}

func readDiskUsage(ctx context.Context, r runc.Runc, sidecar SidecarOpts, opts Opts) (*DiskUsage, error) {
	bundle, err := createBundle(ctx, r, sidecar, opts, "df", "-Pk", mountpointInContainer)
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
			level := zerolog.WarnLevel
			if errors.Is(err, runc.ErrContainerNotFound) {
				level = zerolog.DebugLevel
			}
			log.WithLevel(level).Str("id", bundle.ContainerId()).Err(err).Msg("failed to delete container")
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to read disk usage: %w: %s", err, errb.String())
	}

	usage, err := CalculateDiskUsage(bytes.NewReader(outb.Bytes()))
	if err != nil {
		log.Warn().Err(err).Msg("failed to calculate disk usage")
		return nil, err
	}
	log.Trace().Msgf("Disk usage: %v", usage)
	return &usage, nil
}

func CalculateDiskUsage(r io.Reader) (DiskUsage, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return DiskUsage{}, err
	}
	if len(lines) < 2 {
		return DiskUsage{}, fmt.Errorf("failed to parse df output: %v", lines)
	}
	lines[0] = strings.ReplaceAll(lines[0], "Mounted on", "Mounted-on")
	log.Trace().Msgf("calculateDiskUsage: %v", lines)
	var keyValueMap = make(map[string]string)
	colNames := deleteEmpty(strings.Split(lines[0], " "))
	log.Trace().Msgf("colNames: %v", colNames)
	colValues := deleteEmpty(strings.Split(lines[1], " "))
	log.Trace().Msgf("colValues: %v", colValues)
	//remove empty string
	for idx, colValue := range colValues {
		if colValue != "" && idx < len(colNames) {
			keyValueMap[strings.ToLower(colNames[idx])] = colValue
		}
	}
	log.Trace().Msgf("keyValueMap: %v", keyValueMap)
	if len(keyValueMap) == 0 {
		return DiskUsage{}, fmt.Errorf("failed to parse df output: %v", lines)
	}
	_, ok1k := keyValueMap["1k-blocks"]
	_, ok1024 := keyValueMap["1024-blocks"]
	if !ok1k && !ok1024 {
		return DiskUsage{}, fmt.Errorf("failed to parse 1k-blocks of df output: %v", lines)
	}
	if _, ok := keyValueMap["used"]; !ok {
		return DiskUsage{}, fmt.Errorf("failed to parse used of df output: %v", lines)
	}
	_, okAvailable := keyValueMap["available"]
	_, okAvail := keyValueMap["avail"]
	if !okAvailable && !okAvail {
		return DiskUsage{}, fmt.Errorf("failed to parse available of df output: %v", lines)
	}
	var capacity int64
	var err error
	if ok1k {
		capacity, err = strconv.ParseInt(keyValueMap["1k-blocks"], 10, 64)
	} else {
		capacity, err = strconv.ParseInt(keyValueMap["1024-blocks"], 10, 64)
	}
	if err != nil {
		return DiskUsage{}, err
	}
	used, err := strconv.ParseInt(keyValueMap["used"], 10, 64)
	if err != nil {
		return DiskUsage{}, err
	}
	var available int64
	if okAvailable {
		available, err = strconv.ParseInt(keyValueMap["available"], 10, 64)
	} else {
		available, err = strconv.ParseInt(keyValueMap["avail"], 10, 64)
	}
	if err != nil {
		return DiskUsage{}, err
	}
	result := DiskUsage{
		Capacity:  capacity,
		Used:      used,
		Available: available,
	}
	return result, nil
}

func deleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
