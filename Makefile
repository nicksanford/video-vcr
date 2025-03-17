OS = $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH = $(shell uname -m)
BIN_OUTPUT_PATH = bin/$(OS)-$(ARCH)

$(BIN_OUTPUT_PATH)/vcr: *.go cmd/vcr/*.go server/*.go client/*.go
	go build -o $(BIN_OUTPUT_PATH)/vcr ./cmd/vcr/main.go
