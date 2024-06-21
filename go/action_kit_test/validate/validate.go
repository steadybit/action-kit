// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH

package validate

import (
	"errors"
	"github.com/go-resty/resty/v2"
	"github.com/steadybit/action-kit/go/action_kit_test/client"
)

func ValidateEndpointReferences(path string, restyClient *resty.Client) error {
	c := client.NewActionClient(path, restyClient)
	var allErr []error

	list, err := c.ListActions()
	if err != nil {
		allErr = append(allErr, err)
	}

	for _, action := range list.Actions {
		_, err := c.DescribeAction(action)
		allErr = append(allErr, err)
	}

	return errors.Join(allErr...)
}
