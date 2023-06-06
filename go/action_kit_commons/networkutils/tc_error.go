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
	ignoreErrorsTcAdd = []string{
		strings.ToLower("Error: Exclusivity flag on, cannot modify."),
		strings.ToLower("RTNETLINK answers: File exists"),
	}
	ignoreErrorsTcDelete = []string{
		strings.ToLower("Error: Failed to find qdisc with specified classid."),
		strings.ToLower("Error: Parent Qdisc doesn't exists."),
		strings.ToLower("Error: Invalid handle."),
	}
)

type TcError struct {
	Msg      string
	Lineno   int
	Filename string
}

type TcBatchError struct {
	Errors []TcError
}

var (
	_ error = &TcError{}
	_ error = &TcBatchError{}
)

func (t *TcError) Error() string {
	return fmt.Sprintf("%s\nCommand failed %s:%d", t.Msg, t.Filename, t.Lineno)
}

func (t *TcBatchError) Error() string {
	var sb strings.Builder
	for _, err := range t.Errors {
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

func ParseTcBatchError(r io.Reader) error {
	var msg strings.Builder
	var errs []TcError

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

			errs = append(errs, TcError{Msg: msg.String(), Lineno: lineno, Filename: filename})
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
	return &TcBatchError{Errors: errs}
}

func FilterTcBatchErrors(err error, mode Mode, cmds []string) error {
	tcBatchErr := new(TcBatchError)
	if !errors.As(err, &tcBatchErr) {
		return err
	}

	var errs []TcError
	var ignoreErrors []string
	switch mode {
	case ModeAdd:
		ignoreErrors = ignoreErrorsTcAdd
	case ModeDelete:
		ignoreErrors = ignoreErrorsTcDelete
	}

	for _, e := range tcBatchErr.Errors {
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
	return &TcBatchError{Errors: errs}
}

func contains(ignoreErrors []string, msg string) bool {
	for _, ignoreError := range ignoreErrors {
		if strings.Contains(msg, ignoreError) {
			return true
		}
	}
	return false
}
