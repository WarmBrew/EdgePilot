#!/bin/bash
set -e

AGENT_NAME="robot-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/robot-agent"
SERVICE_FILE="agent.service"

# Detect platform
ARCH=$(uname -m)
case $ARCH in
    aarch64|arm64)
        BINARY="${AGENT_NAME}-arm64"
        ;;
    x86_64|amd64)
        BINARY="${AGENT_NAME}-amd64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Detected architecture: $ARCH"

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

# Create config directory
mkdir -p $CONFIG_DIR
if [ $? -ne 0 ]; then
    echo "Failed to create config directory: $CONFIG_DIR"
    exit 1
fi

# Copy binary
if [ -f "dist/$BINARY" ]; then
    cp "dist/$BINARY" "$INSTALL_DIR/$AGENT_NAME"
    chmod +x "$INSTALL_DIR/$AGENT_NAME"
else
    echo "Binary not found: dist/$BINARY"
    echo "Run 'make cross-compile' first"
    exit 1
fi

# Copy config template if not exists
if [ ! -f "$CONFIG_DIR/agent.env" ]; then
    cp agent.env.example "$CONFIG_DIR/agent.env"
    echo "Please edit $CONFIG_DIR/agent.env with your settings"
fi

# Install systemd service
if [ ! -f "$SERVICE_FILE" ]; then
    echo "Service file not found: $SERVICE_FILE"
    exit 1
fi

cp $SERVICE_FILE /etc/systemd/system/robot-agent.service
systemctl daemon-reload
systemctl enable robot-agent
systemctl start robot-agent

echo ""
echo "Agent installed successfully!"
echo "Status: systemctl status robot-agent"
echo "Logs: journalctl -u robot-agent -f"
echo "Config: $CONFIG_DIR/agent.env"
