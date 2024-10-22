#!/usr/bin/env bash

#
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2022 Steadybit GmbH
#

set -eo pipefail

go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
oapi-codegen -config generator-config.yml -o action_kit_api.go ../../openapi/spec.yml