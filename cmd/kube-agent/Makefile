IMAGE_REPO=ghcr.io/oursky/github-actions-manager/kube-agent
TAG?=latest

.PHONY: build
build:
	go build -o kube-agent .

.PHONY: push
push:
	oras push "${IMAGE_REPO}:git-$$(git rev-parse --short=10 HEAD)" \
		--manifest-config /dev/null:text/plain \
		./kube-agent
