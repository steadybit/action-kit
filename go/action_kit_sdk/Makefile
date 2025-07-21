# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	go fmt ./...
	go mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	gofmt -l .
	go vet ./...
ifneq ($(GOOS),windows)
	go run honnef.co/go/tools/cmd/staticcheck@v0.6.1 -checks=all,-ST1000,-U1000,-ST1003 ./...
endif
	go test -race -vet=off -coverprofile=coverage.out -run SDK ./...
	go mod verify
