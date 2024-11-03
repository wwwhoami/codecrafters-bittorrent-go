.PHONY: audit test


## audit: tidy dependencies and format, vet and test all code
audit:
	@echo "Tidying and verifying modules..."
	go mod tidy
	@echo "Formatting code..."
	go fmt ./...
	@echo "Vetting code..."
	go vet ./...
	@echo "Linting code..."
	-golangci-lint run

test:
	@echo "Running tests..."
	./test.sh

test-all:
	@echo "Running all tests..."
	./test.sh --all
