FROM golang:1.18-bullseye AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . ./
RUN go build -o /kube-controller ./cmd/kube-controller

FROM gcr.io/distroless/base-debian11

WORKDIR /
COPY --from=build /kube-controller /kube-controller
EXPOSE 80
USER nonroot:nonroot

ENTRYPOINT ["/kube-controller"]
