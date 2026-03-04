.PHONY: run build install clean fmt vet

# Build and run immediately
run:
	go run .

# Compile to binary
build:
	go build -o structsh .

# Install to $GOPATH/bin
install:
	go install .

# Format all source files
fmt:
	gofmt -w .

# Vet for common issues
vet:
	go vet ./...

clean:
	rm -f structsh
