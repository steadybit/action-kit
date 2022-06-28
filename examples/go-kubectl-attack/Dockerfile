# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.16-buster AS build

WORKDIR /app

COPY go.mod ./
#COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /go-kubectl-attack

##
## Runtime
##
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /go-kubectl-attack /go-kubectl-attack

EXPOSE 8083

USER nonroot:nonroot

ENTRYPOINT ["/go-kubectl-attack"]
