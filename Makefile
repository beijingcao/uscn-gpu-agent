.PHONY: fmt lint test build smoke release
fmt:
	gofmt -w ./cmd ./internal
lint:
	go vet ./...
test:
	go test -race ./...
build:
	go build -trimpath -buildvcs=false -o dist/uscn-gpu-agent ./cmd/uscn-gpu-agent
smoke: build
	./dist/uscn-gpu-agent version
release:
	python3 scripts/build-release.py --out dist/release
