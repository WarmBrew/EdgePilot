#!/bin/bash
set -e

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit 1
fi

echo "Stopping robot-agent service..."
systemctl stop robot-agent 2>/dev/null || true

echo "Disabling robot-agent service..."
systemctl disable robot-agent 2>/dev/null || true

echo "Removing systemd service file..."
rm -f /etc/systemd/system/robot-agent.service
systemctl daemon-reload

echo "Removing binary..."
rm -f /usr/local/bin/robot-agent

echo "Agent uninstalled successfully"
