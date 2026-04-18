<template>
  <div class="terminal-page">
    <div class="terminal-header">
      <div class="terminal-info">
        <span class="status-dot" :class="connected ? 'online' : 'offline'"></span>
        <span class="terminal-name">{{ deviceName }} </span>
        <span class="terminal-id" v-if="deviceId">{{ deviceId.slice(0, 8) }}</span>
      </div>
      <div class="terminal-actions">
        <span class="session-info" v-if="sessionId">Session: {{ sessionId.slice(0, 8) }}</span>
        <button class="btn btn-ghost" :disabled="!connected" @click="handleDisconnect">
          <svg viewBox="0 0 20 20" fill="currentColor">
            <path d="M5.75 2a.75.75 0 01.75.75V4h7V2.75a.75.75 0 011.5 0V4h.25A2.75 2.75 0 0118 6.75v6.5A2.75 2.75 0 0115.25 16h-.25v1.25a.75.75 0 01-1.5 0V16h-7v1.25a.75.75 0 01-1.5 0V16H4.75A2.75 2.75 0 012 13.25v-6.5A2.75 2.75 0 014.75 4H5V2.75A.75.75 0 015.75 2zM5 5.5h-.25A1.25 1.25 0 003.5 6.75v6.5a1.25 1.25 0 001.25 1.25h10.5A1.25 1.25 0 0016.5 13.25v-6.5A1.25 1.25 0 0015.25 5.5H5z" />
          </svg>
          Disconnect
        </button>
        <router-link :to="{ name: 'Devices' }" class="btn btn-ghost">
          <svg viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M17 10a.75.75 0 01-.75.75H5.612l4.158 3.96a.75.75 0 11-1.04 1.08l-5.5-5.25a.75.75 0 010-1.08l5.5-5.25a.75.75 0 111.04 1.08L5.612 9.25h10.638A.75.75 0 0117 10z" clip-rule="evenodd" />
          </svg>
          Back
        </router-link>
      </div>
    </div>

    <div class="terminal-container" ref="terminalRef"></div>

    <div v-if="connecting" class="connecting-overlay">
      <div class="connecting-card">
        <div class="connecting-spinner"></div>
        <h3>Connecting to device...</h3>
        <p>Establishing secure terminal session</p>
      </div>
    </div>

    <div v-if="error" class="error-overlay">
      <div class="connecting-card">
        <svg viewBox="0 0 20 20" fill="currentColor" class="error-icon">
          <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" clip-rule="evenodd" />
        </svg>
        <h3>Connection Failed</h3>
        <p>{{ error }}</p>
        <button class="btn btn-primary" @click="connectTerminal">Retry</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import { terminalApi } from '@/api/terminal'

const route = useRoute()
const terminalRef = ref<HTMLElement | null>(null)
const deviceId = computed(() => route.params.deviceId as string)

const connected = ref(false)
const connecting = ref(false)
const error = ref('')
const sessionId = ref('')
const deviceName = ref('unknown')

let terminal: Terminal | null = null
let fitAddon: FitAddon | null = null
let ws: WebSocket | null = null

async function connectTerminal() {
  error.value = ''
  connecting.value = true

  const token = localStorage.getItem('access_token')
  if (!token) {
    error.value = 'Not authenticated'
    connecting.value = false
    return
  }

  try {
    const sessionRes = await terminalApi.createSession(deviceId.value)
    sessionId.value = sessionRes.session_id
    deviceName.value = deviceId.value.slice(0, 12)
  } catch (e: any) {
    error.value = e.response?.data?.message || 'Failed to create terminal session'
    connecting.value = false
    return
  }

  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${protocol}://${window.location.host}/ws/terminal/${sessionId.value}?token=${token}`

  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: "'JetBrains Mono', 'SF Mono', Monaco, 'Cascadia Code', 'Fira Code', monospace",
    theme: {
      background: '#0f172a',
      foreground: '#e2e8f0',
      cursor: '#3b82f6',
      selectionBackground: 'rgba(59, 130, 246, 0.3)',
      black: '#0f172a',
      red: '#ef4444',
      green: '#10b981',
      yellow: '#f59e0b',
      blue: '#3b82f6',
      magenta: '#a855f7',
      cyan: '#06b6d4',
      white: '#e2e8f0',
      brightBlack: '#334155',
      brightRed: '#f87171',
      brightGreen: '#34d399',
      brightYellow: '#fbbf24',
      brightBlue: '#60a5fa',
      brightMagenta: '#c084fc',
      brightCyan: '#22d3ee',
      brightWhite: '#f8fafc',
    },
    allowProposedApi: true,
  })

  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)

  terminal.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      const msg = JSON.stringify({
        type: 'input',
        payload: { data: btoa(data) },
      })
      ws.send(msg)
    }
  })

  terminal.onResize(({ cols, rows }) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({
        type: 'resize',
        payload: { cols, rows },
      }))
    }
  })

  terminal.open(terminalRef.value)
  fitAddon.fit()

  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    connected.value = true
    connecting.value = false
    terminal?.writeln('\x1b[32m🚀 Connected to device\x1b[0m\r\n')
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      if (msg.type === 'output' && msg.payload?.data) {
        terminal?.write(atob(msg.payload.data))
      } else if (msg.type === 'close') {
        terminal?.writeln('\r\n\x1b[31mConnection closed\x1b[0m\r\n')
        connected.value = false
      } else if (msg.type === 'error') {
        terminal?.writeln(`\r\n\x1b[31mError: ${msg.payload?.message}\x1b[0m\r\n`)
      } else if (msg.type === 'session_created' && msg.payload?.session_id) {
        sessionId.value = msg.payload.session_id
      }
    } catch {
      terminal?.write(event.data)
    }
  }

  ws.onerror = () => {
    error.value = 'Failed to connect to device'
    connecting.value = false
  }

  ws.onclose = () => {
    connected.value = false
  }
}

async function handleDisconnect() {
  if (ws) {
    ws.send(JSON.stringify({ type: 'close' }))
    ws.close()
  }
  if (sessionId.value) {
    try {
      await terminalApi.closeSession(sessionId.value)
    } catch {
      // Ignore errors on close
    }
  }
  connected.value = false
}

function handleResize() {
  fitAddon?.fit()
}

onMounted(() => {
  connectTerminal()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  ws?.close()
  terminal?.dispose()
})
</script>

<style scoped>
.terminal-page {
  height: calc(100vh - var(--header-height));
  display: flex;
  flex-direction: column;
  margin: -24px;
}

.terminal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
}

.terminal-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot.online {
  background: var(--accent-success);
  box-shadow: 0 0 8px var(--accent-success);
}

.status-dot.offline {
  background: var(--accent-danger);
}

.terminal-name {
  font-weight: 600;
  font-size: 14px;
}

.terminal-id {
  font-size: 12px;
  color: var(--text-muted);
  font-family: 'SF Mono', Monaco, monospace;
}

.terminal-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.session-info {
  font-size: 12px;
  color: var(--text-muted);
  font-family: monospace;
}

.terminal-container {
  flex: 1;
  padding: 0;
}

:deep(.xterm) {
  height: 100%;
}

:deep(.xterm-viewport) {
  background-color: #0f172a !important;
}

.connecting-overlay, .error-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

.connecting-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 40px;
  text-align: center;
  box-shadow: var(--shadow-xl);
}

.connecting-spinner {
  width: 48px;
  height: 48px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin: 0 auto 20px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.connecting-card h3, .error-overlay h3 {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text-primary);
}

.connecting-card p, .error-overlay p {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 20px;
}

.error-icon {
  width: 48px;
  height: 48px;
  color: var(--accent-danger);
  margin-bottom: 16px;
}
</style>
