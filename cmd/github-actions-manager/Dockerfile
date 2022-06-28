FROM node:16.15 as assets
WORKDIR /app
COPY . ./
RUN make -C cmd/github-actions-manager styles

FROM golang:1.18-bullseye AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY --from=assets /app/ ./
RUN go build -o /github-actions-manager ./cmd/github-actions-manager

FROM gcr.io/distroless/base-debian11

WORKDIR /
COPY --from=build /github-actions-manager /github-actions-manager
EXPOSE 80
USER nonroot:nonroot

ENTRYPOINT ["/github-actions-manager"]
