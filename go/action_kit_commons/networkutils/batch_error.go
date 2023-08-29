/*
 * Copyright 2023 steadybit GmbH. All rights reserved.
 */

package networkutils

import (
	"bufio"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io"
	"strconv"
	"strings"
)

var (
	ignoreErrorsBatchAdd = []string{
		strings.ToLower("Error: Exclusivity flag on, cannot modify."),
		strings.ToLower("RTNETLINK answers: File exists"),
	}
	ignoreErrorsBatchDelete = []string{
		strings.ToLower("Error: Failed to find qdisc with specified classid."),
		strings.ToLower("Error: Parent Qdisc doesn't exists."),
		strings.ToLower("Error: Invalid handle."),
		strings.ToLower("Cannot find device"),
		strings.ToLower("RTNETLINK answers: No such file or directory"),
	}
)

type BatchError struct {
	Msg      string
	Lineno   int
	Filename string
}

type BatchErrors struct {
	Cmd    []string
	Errors []BatchError
}

func (t *BatchError) Error() string {
	return fmt.Sprintf("%s\nCommand failed %s:%d", t.Msg, t.Filename, t.Lineno)
}

func (t *BatchErrors) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Command failed %s\n", strings.Join(t.Cmd, " ")))
	for _, err := range t.Errors {
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

func ParseBatchError(cmd []string, r io.Reader) error {
	var msg strings.Builder
	var errs []BatchError

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "Command failed ") {
			filename := "<unknown>"
			lineno := 0
			l := strings.TrimPrefix(scanner.Text(), "Command failed ")

			if i := strings.LastIndex(l, ":"); i > 0 {
				filename = l[:i]
				lineno, _ = strconv.Atoi(l[i+1:])
			}

			errs = append(errs, BatchError{Msg: msg.String(), Lineno: lineno, Filename: filename})
			msg.Reset()
		} else {
			if msg.Len() > 0 {
				msg.WriteString("\n")
			}
			msg.WriteString(scanner.Text())
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &BatchErrors{Cmd: cmd, Errors: errs}
}

func FilterBatchErrors(err error, mode Mode, cmds []string) error {
	batchErrors := new(BatchErrors)
	if !errors.As(err, &batchErrors) {
		return err
	}

	var errs []BatchError
	var ignoreErrors []string
	switch mode {
	case ModeAdd:
		ignoreErrors = ignoreErrorsBatchAdd
	case ModeDelete:
		ignoreErrors = ignoreErrorsBatchDelete
	}

	for _, e := range batchErrors.Errors {
		if !contains(ignoreErrors, strings.ToLower(e.Msg)) {
			errs = append(errs, e)
		} else {
			if e.Lineno-1 >= 0 && e.Lineno-1 < len(cmds) {
				log.Debug().Msgf("Rule '%s' not %sed. Error Ignored: %s", cmds[e.Lineno-1], mode, e.Msg)
			} else {
				log.Debug().Msgf("Error Ignored: %s", e.Msg)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &BatchErrors{Cmd: batchErrors.Cmd, Errors: errs}
}

func contains(ignoreErrors []string, msg string) bool {
	for _, ignoreError := range ignoreErrors {
		if strings.Contains(msg, ignoreError) {
			return true
		}
	}
	return false
}
