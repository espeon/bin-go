# Variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
BINARY_NAME=server
CLI_BINARY_NAME=gbcli

all: build

build: 
	$(GOCMD) mod download
	$(GOBUILD) -o $(BINARY_NAME) -v
	$(GOBUILD) -o $(CLI_BINARY_NAME) -v ./cli/main.go

clean: 
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(CLI_BINARY_NAME)