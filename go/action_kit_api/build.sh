#!/usr/bin/env bash

#
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2022 Steadybit GmbH
#

set -eo pipefail

oapi-codegen -config generator-config.yml ../../openapi/spec.yml > action_kit_api.go

cat extras.go.txt >> action_kit_api.go