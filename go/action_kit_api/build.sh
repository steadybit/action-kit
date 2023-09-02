#!/usr/bin/env bash

#
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2022 Steadybit GmbH
#

set -eo pipefail

go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.13.4
oapi-codegen -config generator-config.yml ../../openapi/spec.yml > action_kit_api.go