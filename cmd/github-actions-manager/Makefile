IMAGE_REPO=ghcr.io/oursky/github-actions-manager
TAG?=latest

.PHONY: styles
styles:
	cd ../../pkg/dashboard && npx tailwindcss -i styles.css -o assets/styles.css $(TW_ARGS)

.PHONY: build
build:
	docker buildx build ../.. -f Dockerfile -t "${IMAGE_REPO}:build" ${BUILDX_ARGS}

.PHONY: push
push:
	docker image tag "${IMAGE_REPO}:build" "${IMAGE_REPO}:${TAG}"
	docker image tag "${IMAGE_REPO}:build" "${IMAGE_REPO}:git-$$(git rev-parse --short=10 HEAD)"
	docker push "${IMAGE_REPO}:${TAG}"
	docker push "${IMAGE_REPO}:git-$$(git rev-parse --short=10 HEAD)"
