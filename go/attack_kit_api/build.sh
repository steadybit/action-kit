#!/usr/bin/env sh

set -eo pipefail

"$GOPATH/bin/oapi-codegen" -config generator-config.yml ../../openapi/spec.yml > attack_kit_api.go

cat extras.go.txt >> attack_kit_api.go