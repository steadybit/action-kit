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
	go run honnef.co/go/tools/cmd/staticcheck@v0.6.1 -checks=all,-ST1000,-U1000,-ST1003 ./...
ifeq ($(OS),Windows_NT)
		go test -race -vet=off -coverprofile=coverage.out ./...
else
		sudo go test -race -vet=off -coverprofile=coverage.out ./...
endif
	go mod verify
