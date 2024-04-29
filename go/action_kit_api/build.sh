#!/usr/bin/env bash

#
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2022 Steadybit GmbH
#

set -eo pipefail

go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@v2.1.0
oapi-codegen -config generator-config.yml ../../openapi/spec.yml > action_kit_api.go