BINARY_NAME=robot-agent
VERSION?=0.1.0
BUILD_DIR=dist

.PHONY: build clean install uninstall cross-compile test

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agent

cross-compile:
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 ./cmd/agent
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-amd64 ./cmd/agent

install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo cp agent.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable robot-agent
	sudo systemctl start robot-agent

uninstall:
	sudo systemctl stop robot-agent
	sudo systemctl disable robot-agent
	sudo rm /etc/systemd/system/robot-agent.service
	sudo rm /usr/local/bin/robot-agent
	sudo systemctl daemon-reload

clean:
	rm -rf $(BUILD_DIR)

test:
	go test -race ./...
