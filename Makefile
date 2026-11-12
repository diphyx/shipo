.PHONY: init
init:
	go mod tidy

.PHONY: format
format:
	gofmt -w .

.PHONY: test
test:
	go test ./...

# custom-gcl is golangci-lint with the diphyx plugin built in. See .custom-gcl.yml.
custom-gcl:
	golangci-lint custom

.PHONY: lint
lint: custom-gcl
	go vet ./...
	./custom-gcl run ./...

.PHONY: build
build:
	go build -o shipo .

# The release archives and checksums.txt, into dist/, as the release workflow builds them.
.PHONY: package
package:
	.github/scripts/package.sh $(or $(TAG),v0.0.0)

# shipo releases itself. See .shipo.
.PHONY: publish
publish:
	go run .
