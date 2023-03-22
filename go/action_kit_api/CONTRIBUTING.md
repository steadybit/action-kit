# Contributing Guidelines

## Installing Required Tools

```sh
go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@2cf7fcf5b26d1a4362e7c300bd65c20f4f6c4298
```

## Executing the Generator

```sh
./build.sh
```

## Releasing

 1. Update `CHANGELOG.md`
 2. Set the tag: `git tag -a go/action_kit_api/v2.0.2 -m go/action_kit_api/v2.0.2`
 3. Push the tag: `git push go/action_kit_api/v2.0.2`