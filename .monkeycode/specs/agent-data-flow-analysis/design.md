# 全系统数据流分析：Agent 上线到设备操作

Feature Name: agent-data-flow-analysis
Updated: 2026-04-18

## 架构总览

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              EdgePilot 系统架构                               │
│                                                                              │
│  ┌─────────────┐      HTTPS/WSS      ┌────────────────────────────────────┐  │
│  │   Browser   │ ◄──────────────────► │         Nginx (web, port 80)      │  │
│  │  (xterm.js) │                      │      反向代理到 API Server         │  │
│  │             │                      └──────────────────┬─────────────────┘  │
│  └─────────────┘                                         │                    │
│         │                                                │                    │
│         │ 1. POST /api/v1/devices/:id/terminal           │                    │
│         │ 2. WS /ws/terminal/:session_id                 │                    │
│         │ 3. REST /api/v1/devices/:id/files/*            │                    │
│         ▼                                                │                    │
│  ┌───────────────────────────────────────────────────────────────────────┐    │
│  │                      Go API Server (port 8080)                         │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────────────┐  │    │
│  │  │  Auth/JWT    │  │  Router      │  │  Handlers                    │  │    │
│  │  │  Middleware  │  │  Gin Engine  │  │  - DeviceHandler             │  │    │
│  │  └──────────────┘  └──────┬───────┘  │  - TerminalHandler           │  │    │
│  │                           │          │  - FileHandler               │  │    │
│  │                           │          │  - HeartbeatHandler          │  │    │
│  │  ┌─────────────────────────────────────────────────────────────────┐│    │
│  │  │ WebSocket Gateway + Hub                                          ││    │
│  │  │  - Hub: deviceID -> Client mapping (内存)                        ││    │
│  │  │  - Gateway: Redis online set + session counter                   ││    │
│  │  │  - SendAndWait: 请求-响应等待器 (respWaiters map)                ││    │
│  │  │  - SubscribeToDevice: 设备消息订阅机制                           ││    │
│  │  └─────────────────────────────────────────────────────────────────┘│    │
│  │                           │                                         │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │    │
│  │  │ Terminal     │  │ File         │  │ Heartbeat                │  │    │
│  │  │ Session Svc  │  │ Service      │  │ Worker (cron)            │  │    │
│  │  │              │  │              │  │ - 60s 检查心跳超时        │  │    │
│  │  │              │  │              │  │ - 90s -> heartbeat_miss   │  │    │
│  │  │              │  │              │  │ - 300s -> offline         │  │    │
│  │  └──────────────┘  └──────────────┘  └──────────────────────────┘  │    │
│  └───────────────────────────────────────────────────────────────────────┘    │
│         │                                                │                    │
│         │ PostgreSQL              ┌──────────────────────┘                    │
│         │ Redis                   │                                          │
│  ┌──────▼───────┐          ┌──────▼───────┐                                  │
│  │ PostgreSQL   │          │    Redis     │                                  │
│  │ - devices    │          │ - ws:online  │                                  │
│  │ - terminal   │          │ - ws:device: │                                  │
│  │   sessions   │          │   *:sessions │                                  │
│  │ - audit_logs │          │ - device:    │                                  │
│  │ - users      │          │   online:*   │                                  │
│  └──────────────┘          │ - device:    │                                  │
│                            │   conn:*     │                                  │
│                            └──────────────┘                                  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐    │
│  │              Agent (边缘设备 - Jetson / x86 / RPi)                     │    │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │    │
│  │  │ WebSocket Client                                                 │  │    │
│  │  │  - 指数退避重连 (1s -> 2s -> 4s -> 8s -> 16s -> 30s max)         │  │    │
│  │  │  - 心跳发送 (30s 间隔)                                           │  │    │
│  │  │  - 消息分发 (handleMessage)                                      │  │    │
│  │  └─────────────────────────────────────────────────────────────────┘  │    │
│  │                            │                                           │    │
│  │  ┌──────────────────┐     │    ┌──────────────────┐                   │    │
│  │  │ PTY Manager      │     │    │ File Handler     │                   │    │
│  │  │ - 创建伪终端      │     │    │ - 列出/读写/删除  │                   │    │
│  │  │ - Shell 白名单    │     │    │ - 路径保护        │                   │    │
│  │  │ - 最大3个会话     │     │    │ - .bak 备份      │                   │    │
│  │  │ - creack/pty     │     │    │ - 路径遍历防护    │                   │    │
│  │  └──────────────────┘     │    └──────────────────┘                   │    │
│  └───────────────────────────│──────────────────────────────────────────┘    │
│                              │                                              │
│              ┌───────────────┼───────────────┐                              │
│              │    ┌───────────▼────┐          │                              │
│              │    │   Shell 进程    │          │   设备文件系统               │
│              │    │  /bin/bash     │          │  /etc, /tmp, /home, ...      │
│              │    └────────────────┘          │                              │
│              │                                │                              │
│              │    WSS (wss://server:8080/ws/agent)                           │
│              └─────────────────────────────────┘                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

## 完整数据流

### 流程 1: Agent 设备上线 (Device Boot & Connect)

```
时间轴: T0 (系统启动) -> T1 (Agent 连接) -> T2 (认证成功) -> T3 (设备在线)

T0: 系统启动
┌─────────────────┐
│ API Server 启动  │
│                  │
│ 1. 初始化 PostgreSQL 连接
│ 2. 初始化 Redis 连接
│ 3. 初始化 Hub:
│    - clients = map[string]*Client{}
│    - register/unregister = chan
│ 4. 初始化 Gateway:
│    - respWaiters = map[string]chan*WSMessage{}
│    - deviceSubscribers = map[string][]func{}
│ 5. 启动 Heartbeat Worker:
│    - cron: 每60秒检查一次
│      * last_heartbeat < now-90s  -> heartbeat_miss
│      * last_heartbeat < now-300s -> offline
│ 6. 注册路由:
│    - POST /api/v1/devices/register
│    - POST /api/v1/devices/verify
│    - GET    /api/v1/devices/ws       (Agent WebSocket)
│    - POST /api/v1/devices/:id/heartbeat
│    - POST /api/v1/devices/:id/terminal
│    - GET    /ws/terminal/:session_id (浏览器 WebSocket)
│    - GET/POST/PUT/DELETE .../files/*
└────────┬────────┘
         │ 等待 Agent 连接
         ▼
T1: Agent 进程启动
┌──────────────────────────────────────────┐
│ Agent 启动 (cmd/agent/main.go)           │
│                                          │
│ 1. 加载 agent.env:
│    - SERVER_URL = wss://server/ws/agent  │
│    - AGENT_TOKEN = "abc123..."           │
│    - DEVICE_ID = "...uuid..."            │
│    - HEARTBEAT_INTERVAL = 30             │
│                                          │
│ 2. 前置校验:                              │
│    - URL 必须以 wss:// 开头              │
│    - AGENT_TOKEN 不能为空                │
│    - DEVICE_ID 不能为空                  │
│                                          │
│ 3. 进入 connectWithRetry 循环:            │
│    状态机: Disconnected -> Connecting    │
│                                          │
│ 4. WebSocket 拨号:                       │
│    dialer.Dial("wss://server/ws/agent")  │
│    HandshakeTimeout: 10s                 │
│    ┌────────────────────────────────┐    │
│    │        Server (HTTP)           │    │
│    │ DeviceHandler.AgentWebSocketAuth│    │
│    │                                │    │
│    │ 1. wsUpgrader.Upgrade HTTP->WS │    │
│    │    CheckOrigin: return true    │    │
│    │                                │    │
│    │ 2. conn.ReadMessage()          │    │
│    │    等待 Agent 发送认证消息      │    │
└────┼──────┬───────────────────────┘    │
     │      │                             │
     │ 5.   │ Agent 发送 auth 消息:       │
     │      │ {                           │
     │      │   "type": "auth",           │
     │      │   "device_id": "...uuid...",│
     │      │   "agent_token": "abc..."   │
     │      │ }                           │
     │      ▼                             │
     │ 6. Server 验证:                    │
     │    - json.Unmarshal -> AgentAuthMsg│
     │    - type 必须是 "auth"            │
     │    - 查 DB: SELECT * FROM devices  │
     │      WHERE id = authMsg.DeviceID   │
     │    - Hash 校验:                    │
     │      sha256(authMsg.AgentToken)    │
     │      vs device.AgentToken          │
     │    - 失败 -> {"error":"auth failed"}│
     │    - 成功继续...                   │
     │                                    │
     │ 7. 注册到 Redis:                   │
     │    SET device:online:{id} "1" 0    │
     │    SET device:conn:{id} {addr} 0   │
     │                                    │
     │ 8. 返回认证成功:                   │
     │    {"status":"ok","device_id":"..."}│
     │                                    │
     │ 9. 注意: AgentWebSocketAuth 在此处  │
     │    进入阻塞 for 循环:              │
     │    for { conn.ReadMessage() }      │
     │    - 此连接由 Handler 独占持有      │
     │    - 消息不经过 Hub               │
     │    - 断开时清理 Redis keys          │
     │                                    │
     │ 10. 此处存在一个架构问题:           │
     │     Agent 认证后 Server 不转发消息  │
     │     给 Hub，而是阻塞在 Handler 中   │
     │     ⚠️ 后续操作(PTY/文件)需另一套   │
     │     WebSocket 连接?               │
     │     实际: 看后续分析...             │
     │                                    │
T2:   │ 认证通过                            │
     │ Agent 状态: Disconnected -> Authenticated
     │                                  ✓ │
     │ 11. Agent 收到 auth response ✓      │
     │     - 检查 status == "ok"           │
     │     - 状态 -> StateAuthenticated    │
     │                                    │
     │ 12. 启动心跳协程:                   │
     │     go sendHeartbeat()              │
     │     Ticker: 30s                     │
     │     发送: {"type":"heartbeat"}      │
     │     ⚠️ 心跳发到哪?                  │
     │        -> 发到 AgentWebSocketAuth   │
     │           阻塞的 ReadMessage 循环   │
     │           但 handler 没处理!        │
     │           只是 ReadMessage() 然后   │
     │           丢弃 -> 心跳丢失!         │
     │           ⚠️ 这是一个发现的 BUG      │
     │                                    │
     │ 13. 启动消息循环:                   │
     │     go messageLoop()                │
     │     - 接收 Server 来的 PTY/文件指令  │
     └────┼────────────────────────────────┘
          │
T3:       │ ⚠️ 问题: Agent 认证后的 WebSocket
          │ 连接由 DeviceHandler 持有, 不在 Hub
          │ 中注册. 后续 PTY/File 操作通过
          │ Gateway.SendMessageToDevice() 发
          │ 消息时查找 Hub.GetClient(deviceID)
          │ 必定返回 nil!
          │
          │ 💡 分析: 系统实际运行逻辑:
          │    Agent 的 WebSocket 连接
          │    与后续操作的 WebSocket 连接
          │    是同一条. DeviceHandler 持有
          │    它, 但 Hub 没有注册 Agent Client.
          │    所以 SendAndWait 找不到 Client!
          │
          │ ⚠️ BUG CONFIRMED:
          │    DeviceHandler.AgentWebSocketAuth
          │    与 Gateway 是独立的 WebSocket
          │    处理路径, 没有共享 Hub 中的
          │    Agent Client.
          │
          │ 📋 修复方案:
          │    方案A: 在认证后将 Agent 注册到 Hub
          │    方案B: 合并 AgentWSS 和 Gateway 处理
          │    方案C: 在 AgentWSS 中复用 Gateway
```

### 流程 2: 用户打开终端 (Open Terminal Session)

```
触发: 用户在浏览器点击 "Terminal" 按钮

Step 1: 创建终端会话 (HTTP POST)
────────────────────────────────
┌─────────────┐        POST /api/v1/devices/:id/terminal
│   Browser   │ ──────────────────────────────────────────┐
│  (前端)      │                                          │
└─────────────┘                                          │
                                                         ▼
                                                ┌────────────────────┐
                                                │ TerminalHandler    │
                                                │ HandleTerminal     │
                                                └────────┬───────────┘
                                                         │
                                                         │ 1. 验证 JWT Token
                                                         │    auth.ValidateToken(token)
                                                         │    提取 userID, role, tenantID
                                                         │
                                                         │ 2. 检查设备在线状态
                                                         │    gw.IsDeviceOnline(deviceID)
                                                         │    -> Hub.GetClient(deviceID)
                                                         │    ⚠️ 此处因流程 1 的 BUG 会失败
                                                         │    假设已修复...
                                                         │
                                                         │ 3. 创建 TerminalSession 记录到 DB
                                                         │    INSERT INTO terminal_sessions
                                                         │    status = 'pending'
                                                         │
                                                         │ 4. 向 Agent 发送 create_pty
                                                         │    gw.SendAndWait(ctx, deviceID,
                                                         │      "create_pty",
                                                         │      {session_id, user_id,
                                                         │       rows:24, cols:80},
                                                         │      session_id,
                                                         │      timeout: 10s)
                                                         │
                                                         ▼
                                                ┌────────────────────┐
                                                │ Gateway.SendAndWait│
                                                │                    │
                                                │ 1. 检查 Redis:     │
                                                │    SISMEMBER ws:   │
                                                │    online {id}     │
                                                │                    │
                                                │ 2. 注册 respWaiter:│
                                                │    respWaiters[    │
                                                │      session_id    │
                                                │    ] = chan        │
                                                │                    │
                                                │ 3. 发消息到 Agent:  │
                                                │    Hub.GetClient() │
                                                │    -> Client.Send()│
                                                │    -> client.send  │
                                                │       chan         │
                                                │                    │
                                                │ 4. 等待响应:       │
                                                │    select {        │
                                                │    case resp <- ch:│
                                                │      return resp   │
                                                │    case 10s timeout│
                                                │      return error  │
                                                │    }               │
                                                └────────┬───────────┘
                                                         │
                                                         │ create_pty 消息通过 Hub -> Client
                                                         │ -> WritePump -> WebSocket -> Agent
                                                         ▼
                                            ┌──────────────────────────┐
                                            │   Agent messageLoop      │
                                            │                          │
                                            │ handleMessage(msg):      │
                                            │   case "create_pty":     │
                                            │     handlePTYCreate()    │
                                            │                          │
                                            │ PTYManager.CreateSession │
                                            │ ┌────────────────────┐   │
                                            │ │ 1. 解析 payload     │   │
                                            │ │ 2. tryAcquire()    │   │
                                            │ │    semaphore <= 3  │   │
                                            │ │ 3. validateShell() │   │
                                            │ │    白名单检查       │   │
                                            │ │ 4. exec.Command()  │   │
                                            │ │    SysProcAttr:    │   │
                                            │ │      Pdeathsig:SIGK│   │
                                            │ │      Setpgid:true  │   │
                                            │ │ 5. pty.Start(cmd)  │   │
                                            │ │ 6. pty.Setsize()   │   │
                                            │ │    Cols:80,Rows:24 │   │
                                            │ │ 7. 写入 sessions   │   │
                                            │ │    map[session_id] │   │
                                            │ │ 8. send PTYReady:  │   │
                                            │ │    {session_id,    │   │
                                            │ │     PtyPath}       │   │
                                            │ │ 9. go readLoop()   │   │
                                            │ │    4KB buffer,     │   │
                                            │ │    base64 编码     │   │
                                            │ │    发送 pty_output │   │
                                            │ │ 10. go waitProcess │   │
                                            │ │     等待 shell 退出 │   │
                                            │ └────────────────────┘   │
                                            │                          │
                                            │ 发送方式:                │
                                            │   ws.WriteJSON(msg)     │
                                            │   -> 直接走 Agent 的     │
                                            │   WebSocket 连接        │
                                            └──────────┬───────────────┘
                                                       │
                                                       │ pty_ready 消息:
                                                       │ {"type":"pty_ready",
                                                       │  "payload":{
                                                       │   "success":true,
                                                       │   "pty_path":"/dev/pts/0"
                                                       │  }}
                                                       │ 通过 Agent WS 发回 Server
                                                       ▼
                                            ┌──────────────────────────┐
                                            │ Server ReadPump           │
                                            │ (Hub 的 Client.ReadPump)  │
                                            │                          │
                                            │ 收到消息后:              │
                                            │ 1. json.Unmarshal ->    │
                                            │    WSMessage             │
                                            │ 2. type == "pty_ready"?  │
                                            │    非 heartbeat ->       │
                                            │    hub.messageHandler()  │
                                            │ 3. Gateway.handleDevMsg │
                                            │ 4. tryResolveResponse() │
                                            │    msg.Session != ""    │
                                            │    -> respWaiters[id]   │
                                            │    -> chan <- msg       │
                                            └──────────┬───────────────┘
                                                       │
                                                       │ Gateway.SendAndWait 收到响应 ✓
                                                       ▼
                                            ┌──────────────────────────┐
                                            │ TerminalSessionService   │
                                            │ CreateSession            │
                                            │                          │
                                            │ 1. 解析 PTYResponse      │
                                            │ 2. 检查 ptyResp.Success  │
                                            │ 3. 更新 DB:              │
                                            │    UPDATE terminal_sessions
                                            │    SET status = 'active' │
                                            │        pty_path = '/dev/ │
                                            │          pts/0'          │
                                            │ 4. 写入 Redis 缓存:      │
                                            │    terminal:session:id → │
                                            │    {user,device,pty_path}│
                                            │ 5. 写入审计日志:         │
                                            │    action = 'terminal_open'
                                            │ 6. 返回 CreateSessionResult
                                            └──────────┬───────────────┘
                                                       │
                                                       │ HTTP 响应:
                                                       │ {
                                                       │   "session_id": "...",
                                                       │   "device_id": "...",
                                                       │   "pty_path": "/dev/pts/0",
                                                       │   "status": "active"
                                                       │ }
                                                       ▼

Step 2: 浏览器连接 WebSocket
─────────────────────────────
┌─────────────┐       WS /ws/terminal/:session_id?token=xxx
│   Browser   │ ──────────────────────────────────────────┐
│  (xterm.js) │                                          │
│             │ 1. new Terminal() (xterm.js)              │
│             │ 2. new WebSocket(wsUrl)                   │
└─────────────┘                                          │
                                                         ▼
                                                ┌────────────────────┐
                                                │ TerminalHandler    │
                                                │ HandleTerminalWS   │
                                                └────────┬───────────┘
                                                         │
                                                         │ 1. 解析 query token
                                                         │    auth.ValidateToken(token)
                                                         │
                                                         │ 2. 查询 session from DB:
                                                         │    WHERE id = :session_id
                                                         │    AND user_id = :user_id
                                                         │    status 必须是 'active'
                                                         │
                                                         │ 3. 检查终端会话数限制:
                                                         │    Redis Scan: terminal:active:*
                                                         │    同设备 <= 3 个 active 会话
                                                         │
                                                         │ 4. termUpgrader.Upgrade HTTP->WS
                                                         │
                                                         │ 5. 创建 browserConn 结构体:
                                                         │    {sessionID, userID, role,
                                                         │     deviceID, clientIP, conn,
                                                         │     ctx, cancel, lastActivity,
                                                         │     closed}
                                                         │
                                                         │ 6. 缓存到 Redis:
                                                         │    terminal:active:{session_id}
                                                         │    = deviceID (TTL: 60min)
                                                         │
                                                         │ 7. 写入审计日志:
                                                         │    action: 'terminal_open'
                                                         │
                                                         │ 8. 订阅设备消息:
                                                         │    gw.SubscribeToDevice(
                                                         │      deviceID,
                                                         │      func(msg) {
                                                         │        routeDeviceMessage(
                                                         │          sessionID, msg)
                                                         │      })
                                                         │
                                                         │ 9. 进入 forwardLoop:
                                                         │    go browserReadLoop(bc)  ← 读浏览器
                                                         │    go monitorIdle(bc)      ← 超时监控
                                                         │    select { <- ctx.Done }  ← 等待结束

Step 3: 终端数据双向转发 (Terminal Data Relay)
────────────────────────────────────────────
浏览器输入 -> Agent:

  Browser (xterm.js)
       │
       │ WS TextMessage:
       │ {"type":"input",
       │  "payload":{"data":"base64(input_data)"}}
       │
       ▼
  TerminalHandler.browserReadLoop()
       │
       │ 1. bc.conn.ReadMessage()
       │ 2. json.Unmarshal -> WSMessage
       │ 3. type == "input" (PTYInput)
       │ 4. handleBrowserInput(bc, msg):
       │    a. 解析 payload.data -> base64 decode
       │    b. CheckCommand(input, role):
       │       - 危险命令 -> blocked
       │         -> sendToBrowser(bc, "blocked")
       │         -> 写入审计日志 (command_blocked)
       │       - 敏感命令 -> 需确认
       │         -> sendConfirmRequest()
       │         -> 弹窗等待用户确认
       │         -> 用户确认后再转发
       │       - 普通命令 -> 直接转发
       │    c. forwardInput(bc, data, input):
       │       gw.SendMessageToDevice(
       │         deviceID,
       │         "pty_input",
       │         {session_id, user_id,
       │          data: base64_encoded})
       │
       ▼
  Gateway.SendMessageToDevice(deviceID, ...)
       │
       │ 检查在线: isDeviceOnline() -> Redis SISMEMBER
       │ Hub.GetClient(deviceID) -> Client.Send(msg)
       │ -> client.send chan -> WritePump -> Agent WS
       │
       ▼
  Agent Client.messageLoop()
       │
       │ handleMessage:
       │   case "pty_input":
       │     parse WritePayload
       │     base64 decode
       │     PTYManager.WriteToPTY(sessionID, data)
       │
  PTYManager
       │
       │ session.Pty.Write(data) -> 写入 PTY 主设备
       │ -> shell 进程读取并执行
       │
       ▼
  Agent PTYManager.readLoop()
       │
       │ session.Pty.Read(buf) <- 读取 PTY 输出
       │ base64 encode
       │ sendJSON(PTYOutput{
       │   SessionID, Data:base64(output)})
       │
       ▼
  Agent -> Server (Agent WS)
       │
       │ WS TextMessage:
       │ {"type":"pty_output",
       │  "payload":{"session_id":"...","data":"..."}}
       │
       ▼
  Server ReadPump -> Hub -> messageHandler
       │
       │ Gateway.handleDeviceMessage()
       │ -> tryResolveResponse: no session in message
       │ -> notifyDeviceSubscribers(deviceID, msg)
       │
       ▼
  TerminalHandler.routeDeviceMessage(sessionID, msg)
       │
       │ 1. activeSessions.Load(sessionID) -> browserConn
       │ 2. type == "pty_output":
       │    解析 payload {session_id, data}
       │    sendToBrowser(bc, "output", {data})
       │
       ▼
  Browser (xterm.js)
       │
       │ WS Message:
       │ {"type":"output","payload":{"data":"..."}}
       │ terminal.write(atob(msg.payload.data))
       │ -> 终端显示输出
```

### 流程 3: 文件操作 (File Operations)

```
触发: 用户在 Files 页面浏览/编辑文件

┌─────────────┐         GET /api/v1/devices/:id/files?path=/tmp
│   Browser   │ ──────────────────────────────────────────────┐
│ (Files页面) │                                              │
└─────────────┘                                              │
                                                             ▼
                                                  ┌──────────────────────┐
                                                  │ FileHandler          │
                                                  │ ListFiles            │
                                                  └────────┬─────────────┘
                                                           │
                                                           │ 1. 获取 userID, deviceID, path
                                                           │ 2. FileService.ListDir(ctx, ...)
                                                           ▼
                                                  ┌──────────────────────┐
                                                  │ FileService.ListDir  │
                                                  │                      │
                                                  │ 1. 检查设备在线       │
                                                  │    gw.IsDeviceOnline │
                                                  │ 2. 构造请求:         │
                                                  │    gw.SendAndWait(   │
                                                  │      ctx, deviceID,  │
                                                  │      "list_dir",     │
                                                  │      {path},         │
                                                  │      fileSessionID,  │
                                                  │      timeout:10s)    │
                                                  └────────┬─────────────┘
                                                           │
                                                           ▼
                                              ┌──────────────────────────┐
                                              │ Agent Client.handleList │
                                              │ Dir(payload)             │
                                              │                          │
                                              │ FileHandler.ListDir(path)│
                                              │ ┌────────────────────┐   │
                                              │ │ 1. sanitizedPath() │   │
                                              │ │    - 清理 ".."     │   │
                                              │ │    - 绝对路径检查   │   │
                                              │ │ 2. IsProtectedPath │   │
                                              │ │    - /etc/shadow ✓ │   │
                                              │ │    - /proc/*    ✓  │   │
                                              │ │    - /sys/*     ✓  │   │
                                              │ │    - /dev/*     ✓  │   │
                                              │ │    - /boot/*    ✓  │   │
                                              │ │ 3. os.ReadDir(path)│   │
                                              │ │ 4. 构造 FileInfo[] │   │
                                              │ │    {name, path,    │   │
                                              │ │     size, is_dir,  │   │
                                              │ │     mod_time, mode}│   │
                                              │ │ 5. 目录排在前面     │   │
                                              │ │ 6. 发送 list_dir_  │   │
                                              │ │    resp            │   │
                                              │ └────────────────────┘   │
                                              └──────────┬───────────────┘
                                                         │
                                                         ▼
                                              Agent -> Server -> Gateway
                                                       │
                                                       │ 通过 respWaiters[fileSessionID]
                                                       │ 返回到 FileService.ListDir
                                                       ▼
                                              FileService 将返回结果
                                              -> FileHandler.ListFiles
                                              -> JSON response to Browser
```

### 流程 4: 心跳保活 (Heartbeat Loop)

```
┌────────────────────────────────────────────────────────────────────┐
│ Agent 端 (每 30 秒)                                                │
│                                                                    │
│ Client.sendHeartbeat()                                             │
│ ┌──────────────────────────────────────────┐                       │
│ │ Ticker: 30s                              │                       │
│ │ conn.WriteJSON({"type":"heartbeat"})     │                       │
│ │ -> 通过 Agent WS 发给 Server             │                       │
│ │ -> ⚠️ 当前实现问题:                      │                       │
│ │    发到 AgentWSS Handler 的 ReadMessage  │                       │
│ │    循环, 但 handler 只是 read + 丢弃!    │                       │
│ │    不会更新设备心跳时间戳!               │                       │
│ │    ⚠️ BUG: 心跳消息被丢弃               │                       │
│ └──────────────────────────────────────────┘                       │
└────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────┐
│ Server 端 - HeartbeatWorker (每 60 秒 cron)                        │
│                                                                    │
│ checkHeartbeats()                                                  │
│ ┌──────────────────────────────────────────┐                       │
│ │ 1. markAsHeartbeatMiss:                  │                       │
│ │    WHERE status IN ('online','hb_miss')  │                       │
│ │      AND last_heartbeat < now - 90s      │                       │
│ │      AND last_heartbeat >= now - 300s    │                       │
│ │    -> SET status = 'heartbeat_miss'      │                       │
│ │                                          │                       │
│ │ 2. markAsOffline:                        │                       │
│ │    WHERE status = 'heartbeat_miss'       │                       │
│ │      AND last_heartbeat < now - 300s     │                       │
│ │    -> SET status = 'offline'             │                       │
│ │                                          │                       │
│ │ 3. publishStatusEvent() to Redis PubSub  │                       │
│ │    channel = "device:status:events"      │                       │
│ └──────────────────────────────────────────┘                       │
│                                                                    │
│ ⚠️ 注意: HeartbeatWorker 依赖 DB 中的           │
│    last_heartbeat 字段. 该字段应由              │
│    HeartbeatHandler.HandleHeartbeat 更新.       │
│    但 Agent 心跳发到的是 AgentWSS Handler,      │
│    不经过 HeartbeatHandler!                    │
│    ⚠️ CONFIRMED BUG: 心跳永远不会更新 DB         │
└────────────────────────────────────────────────────────────────────┘
```

## 发现的架构 Bug

### Bug 1: Agent WebSocket 连接不与 Hub 共享

**问题描述**: `DeviceHandler.AgentWebSocketAuth()` 独自持有 Agent 的 WebSocket 连接,
不注册到 Hub 中. 后续 `Gateway.SendMessageToDevice()` 通过 `Hub.GetClient()` 查找
设备连接时必定返回 nil.

**影响**: 所有需要向 Agent 发送消息的操作 (创建 PTY、文件操作) 都无法工作.

**修复方案**:
```go
// 方案 A: 在认证成功后将连接注册到 Hub
func (h *DeviceHandler) AgentWebSocketAuth(c *gin.Context) {
    // ... 认证逻辑 ...
    
    conn.WriteJSON(gin.H{"status": "ok", "device_id": deviceID})
    
    // 创建 Hub Client 并注册
    client := websocket.NewClient(h.hub, conn, deviceID)
    h.hub.RegisterClient(client)
    
    go client.ReadPump()  // ReadPump 会处理消息并分发给 Hub
    go client.WritePump()
    
    <-client.ctx.Done()  // 等待连接断开
    h.hub.UnregisterClient(client)
}
```

### Bug 2: Agent 心跳消息被丢弃

**问题描述**: Agent 发送的 `heartbeat` 消息发到 `AgentWSS Handler`, 该 Handler
只是 `for { ReadMessage() }` 循环, 不处理任何消息内容.

**影响**: 数据库的 `last_heartbeat` 字段永远不会更新, HeartbeatWorker 会在 90 秒
后将所有设备标记为 `heartbeat_miss`, 300 秒后标记为 `offline`.

**修复方案**: 在 `Client.ReadPump()` 中添加心跳处理:
```go
case MessageTypeHeartbeat:
    // 更新 DB 中的 last_heartbeat
    h.db.Model(&Device{}).Where("id = ?", c.deviceID).
        Update("last_heartbeat", time.Now())
    c.sendPong()
```

### Bug 3: Session ID 传递不一致

**问题描述**: Gateway.SendAndWait 使用 sessionID 作为 respWaiters 的 key,
但 Agent 返回消息时需要在 `session` 字段或 payload.session_id 中携带.
当前 Agent 的响应消息格式不统一, 部分类型使用 `session` 字段, 部分使用 payload.

**影响**: tryResolveResponse 可能无法匹配到正确的 waiter, 导致 SendAndWait 超时.

**修复方案**: 统一 Agent 响应消息格式, 确保 `session` 字段在所有响应中都设置.

## Redis 数据结构

| Key 模式 | 类型 | 用途 | TTL |
|---------|------|------|-----|
| `ws:online` | Set | 所有在线设备 ID 集合 | 无 (手动管理) |
| `ws:device:{id}:sessions` | Integer (Incr/Decr) | 设备活跃连接数 | 5 min |
| `terminal:active:{id}` | String | 终端会话 -> 设备映射 | 60 min |
| `terminal:confirm:{id}` | String | 敏感命令确认数据 (JSON) | 30s |
| `terminal:confirm_rate:{id}` | Integer | 确认请求速率限制 | 1 min |
| `device:online:{id}` | String | Agent 在线标记 | 无 |
| `device:conn:{id}` | String | Agent 连接地址 | 无 |

## 安全机制

| 层级 | 位置 | 机制 |
|------|------|------|
| 传输加密 | Agent <-WSS-> Server | 强制 `wss://` 协议 |
| Agent 认证 | AgentWSS Handler | device_id + agent_token Hash 校验 |
| 用户认证 | 浏览器 -> Server | JWT Token (query param) |
| 租户隔离 | JWT Middleware | tenant_id 查询过滤 |
| 角色控制 | RBAC Middleware | admin/operator/viewer 权限 |
| 命令过滤 | CheckCommand() | 危险命令阻止 + 敏感命令二次确认 |
| 路径保护 | Agent IsProtectedPath | 阻止访问系统敏感文件 |
| 路径清理 | Agent sanitizedPath | 防止 `..` 路径遍历 |
| Shell 白名单 | Agent validateShell | 仅允许 bash/sh/zsh |
| 会话限制 | PTY Semaphore | 最大 3 个并发 PTY |
| 速率限制 | Redis confirm_rate | 每分钟最多 5 次确认请求 |
| 超时保护 | Terminal Idle Monitor | 30 分钟无活动自动关闭 |

## 数据流总结

```
Agent 上线:  Agent --WSS--> AgentWSS Handler → Redis (online/conn)
                                        → DB (心跳? ⚠️ BUG: 不更新)

终端操作:    Browser --HTTP--> TerminalHandler --Gateway--> Hub --WSS--> Agent (PTY)
             Browser --WS-->   TerminalHandler ◄─Subscribe─◄ Gateway ◄─WSS─ Agent (output)

文件操作:    Browser --HTTP--> FileHandler --Gateway--SendAndWait--> Hub --WSS--> Agent (fileop)
             Browser ◄──HTTP── FileHandler ◄──respWaiter◄── Gateway ◄──WSS── Agent (resp)

心跳保活:    Agent --WSS--> AgentWSS Handler → ⚠️ 消息被丢弃 (BUG)
             HeartbeatWorker --cron--> DB (last_heartbeat) → 更新 status
```
