# Edge Agent Deployment Guide

## Quick Start

### 1. Build

```bash
cd agent
make cross-compile
```

### 2. Install

```bash
sudo ./install.sh
```

### 3. Configure

Edit `/etc/robot-agent/agent.env`:

```
SERVER_URL=wss://your-server.com/ws/agent
AGENT_TOKEN=your-registration-token
DEVICE_ID=unique-device-id
PLATFORM=jetson
ARCH=arm64
LOG_LEVEL=info
HEARTBEAT_INTERVAL=30
```

### 4. Restart

```bash
sudo systemctl restart robot-agent
```

### 5. Check Status

```bash
sudo systemctl status robot-agent
sudo journalctl -u robot-agent -f
```

## Supported Platforms

- Linux ARM64 (Jetson Nano, Jetson Xavier, Raspberry Pi 4/5)
- Linux AMD64 (x86_64 desktops/servers)

## Manual Start

```bash
./dist/robot-agent  # Use agent.env in current directory
```
