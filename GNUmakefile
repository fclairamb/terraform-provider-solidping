default: build

build:
	go build ./...

install:
	go install .

fmt:
	gofmt -s -w .

lint:
	golangci-lint run ./...

# Unit tests only: the acceptance tests are skipped without TF_ACC.
test:
	go test ./... -count=1

# Acceptance tests, against the SolidPing server named by SOLIDPING_ENDPOINT / SOLIDPING_ORG
# (see README.md, "Developing").
testacc:
	TF_ACC=1 go test ./... -count=1 -timeout 30m -v

# Regenerate docs/ from the schemas and examples/.
docs:
	go generate ./...

.PHONY: default build install fmt lint test testacc docs
