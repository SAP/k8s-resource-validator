test: ## Tests .
	go test ./pkg/... -coverprofile cover.out

build: ## Build sample app
	go build -o bin/k8s-resource-validator ./cmd/sample.go
