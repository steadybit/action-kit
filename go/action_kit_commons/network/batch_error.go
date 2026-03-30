// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package network

import (
	"bufio"
	"errors"
	"fmt"
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
	tcErrorKernelConfig = []string{
		strings.ToLower("Error: Specified qdisc not found."),
		strings.ToLower("Error: Specified qdisc kind is unknown."),
	}
)

type batchError struct {
	Msg      string
	Lineno   int
	Filename string
}

type batchErrors struct {
	Cmd    []string
	Errors []batchError
}

func (t *batchError) Error() string {
	return fmt.Sprintf("%s\nCommand failed %s:%d", t.Msg, t.Filename, t.Lineno)
}

func (t *batchErrors) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Command failed %s\n", strings.Join(t.Cmd, " ")))
	for _, err := range t.Errors {
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

func parseBatchError(cmd []string, r io.Reader) error {
	var msg strings.Builder
	var errs []batchError

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

			errs = append(errs, batchError{Msg: msg.String(), Lineno: lineno, Filename: filename})
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
	return &batchErrors{Cmd: cmd, Errors: addTcErrorHints(errs)}
}

func addTcErrorHints(errs []batchError) []batchError {
	for _, e := range errs {
		if contains(tcErrorKernelConfig, strings.ToLower(e.Msg)) {
			return append([]batchError{
				{
					Msg:      "Kernel configuration error. Please check if the required kernel modules are loaded.",
					Lineno:   0,
					Filename: "",
				}, {
					Msg:      "This is expected, for example, when using minikube under Windows with WLS2 (https://github.com/microsoft/WSL/issues/6065).",
					Lineno:   0,
					Filename: "",
				},
			}, errs...)
		}
	}
	return errs
}

func filterBatchErrors(err error, mode Mode, cmds []string) error {
	be := new(batchErrors)
	if !errors.As(err, &be) {
		return err
	}

	var errs []batchError
	var ignoreErrors []string
	switch mode {
	case ModeAdd:
		ignoreErrors = ignoreErrorsBatchAdd
	case ModeDelete:
		ignoreErrors = ignoreErrorsBatchDelete
	}

	for _, e := range be.Errors {
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
	return &batchErrors{Cmd: be.Cmd, Errors: errs}
}

func contains(ignoreErrors []string, msg string) bool {
	for _, ignoreError := range ignoreErrors {
		if strings.Contains(msg, ignoreError) {
			return true
		}
	}
	return false
}
