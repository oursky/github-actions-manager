# syntax=docker/dockerfile:1.4

FROM ubuntu:22.04 AS runner-dl

WORKDIR /runner
RUN apt-get update && apt-get install -y curl
RUN curl -L https://github.com/actions/runner/releases/download/v2.294.0/actions-runner-linux-arm64-2.294.0.tar.gz | \
    tar xzf -

FROM golang:1.18-bullseye AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /kube-agent ./cmd/kube-agent

FROM ubuntu:22.04

WORKDIR /runner

RUN apt-get update && apt-get install -y curl dumb-init sudo
RUN adduser --disabled-password --gecos "" --uid 1000 runner && \
    usermod -aG sudo runner && \
    usermod -aG sudo root && \
    echo "%sudo   ALL=(ALL:ALL) NOPASSWD:ALL" > /etc/sudoers

COPY --from=runner-dl --chown=runner:runner /runner /runner
RUN chown runner:runner /runner
RUN ./bin/installdependencies.sh

RUN apt-get install -y bash jq

USER runner
COPY --from=build /kube-agent /kube-agent

ENTRYPOINT ["dumb-init", "/kube-agent"]
