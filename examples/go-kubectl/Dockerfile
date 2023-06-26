# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.20-bullseye AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /go-kubectl

##
## Runtime
##
FROM debian:bullseye-slim

WORKDIR /

RUN apt-get update && \
  apt-get install -y apt-transport-https ca-certificates curl gpg && \
  curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /etc/apt/trusted.gpg.d/kubernetes-archive.gpg && \
  echo "deb [signed-by=/etc/apt/trusted.gpg.d/kubernetes-archive.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list && \
  apt-get update && \
  apt-get install -y kubectl

COPY --from=build /go-kubectl /go-kubectl

EXPOSE 8083

ENTRYPOINT ["/go-kubectl"]
