.PHONY: all clean build

# Default target
all: clean build

# Build target
build:
	@mkdir -p bin
	@go build -o bin/mmdbmerge

# Clean target
clean:
	@rm -rf bin/

# Test target (optional)
test:
	@go test -v ./... 