# Contributing Guidelines

## Installing Required Tools

```sh
go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@2cf7fcf5b26d1a4362e7c300bd65c20f4f6c4298
```

## Executing the Generator

```sh
"$GOPATH/bin/oapi-codegen" -config generator-config.yml ../../openapi/spec.yml > attack_kit_api.go
```