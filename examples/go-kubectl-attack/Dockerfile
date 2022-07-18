# syntax=docker/dockerfile:1

##
## Build
##
FROM golang:1.18-buster AS build

WORKDIR /app

COPY go.mod ./
#COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /go-kubectl-attack

##
## Runtime
##
FROM ubuntu:latest

WORKDIR /

RUN apt-get update && \
  apt-get install -y apt-transport-https ca-certificates curl && \
  curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg && \
  echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | tee /etc/apt/sources.list.d/kubernetes.list && \
  apt-get update && \
  apt-get install -y kubectl

COPY --from=build /go-kubectl-attack /go-kubectl-attack

EXPOSE 8083

ENTRYPOINT ["/go-kubectl-attack"]
