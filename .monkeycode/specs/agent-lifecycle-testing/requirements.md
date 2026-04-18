# Requirements Document

## Introduction

EdgePilot Agent 是一个运行在边缘设备（如 Jetson、Raspberry Pi、x86 服务器）上的 Go 程序，通过 WebSocket 与安全后端保持长连接，提供远程终端（PTY）、文件管理、系统信息收集和心跳保活功能。本需求文档定义了 Agent 从代码生成、测试、构建到生产部署上线的完整生命周期质量要求。

## Glossary

- **Agent**: 运行在边缘设备上的 Go 客户端程序，通过 WebSocket 与 Server 通信
- **Server**: 中心化的 Go 后端服务，管理设备连接、终端会话、文件操作和审计
- **Server 测试实例**: 启动的真实 Server 实例或 mock 版本，用于 Agent 测试
- **Stage 环境**: 与生产配置一致但使用独立数据库和 Redis 的中间环境
- **Canary**: 灰度部署策略，先向少量设备推送新版本 Agent
- **RTU (Return-To-User)**: Agent 版本发布后，用户在边缘设备上看到的新版本运行状态

## Requirements

### Requirement 1: Agent 构建与交叉编译

**User Story:** AS 运维工程师, I WANT Agent 能够通过一键命令为 arm64 和 amd64 架构生成二进制文件, SO THAT 不同边缘设备都能正确部署。

#### Acceptance Criteria

1. WHEN 执行 `make cross-compile`, THE Agent 构建系统 SHALL 生成 `robot-agent-arm64` 和 `robot-agent-amd64` 两个二进制文件
2. WHEN 构建完成, THE 构建系统 SHALL 为每个二进制文件输出 SHA256 校验和到一个 `checksums.txt` 文件
3. WHEN 构建完成, THE 构建系统 SHALL 输出二进制文件的版本号（取自 git tag 或 VERSION 变量）
4. WHILE 构建过程运行, THE 构建系统 SHALL 使用 `-trimpath` 和 `-ldflags="-s -w"` 参数以减小二进制体积并去除路径信息
5. IF 构建环境缺少必要的 Go 依赖, THE 构建系统 SHALL 在失败前提示具体缺失的模块并输出 `go mod download` 命令

### Requirement 2: Agent 单元测试

**User Story:** AS 开发人员, I WANT Agent 的每个内部模块都能通过 `go test` 独立运行单元测试, SO THAT 代码变更不会引入回归缺陷。

#### Acceptance Criteria

1. WHEN 执行 `make test`, THE 测试系统 SHALL 运行 `go test -race ./...` 并且所有测试用例通过
2. WHILE 新增代码提交到 main 分支, THE CI 管道 SHALL 执行单元测试并要求覆盖率不低于 70%
3. WHEN 测试失败, THE 测试框架 SHALL 输出具体的失败用例名称、行号和失败原因
4. IF 测试中存在数据竞争, THE `-race` 参数 SHALL 检测并报告竞争的 goroutine 位置和变量
5. WHEN 编写新功能的单元测试, THE 开发人员 SHALL 使用表驱动测试（table-driven tests）模式

### Requirement 3: Agent 集成测试（Agent <-> Server WebSocket）

**User Story:** AS 测试工程师, I WANT 能够在一个隔离环境中同时启动 Agent 和 Server 并验证它们的 WebSocket 交互, SO THAT 确保端到端通信协议正确。

#### Acceptance Criteria

1. WHEN 启动集成测试, THE 测试框架 SHALL 自动启动一个 Server 实例（使用内存数据库或 Dockerized PostgreSQL+Redis）和一个 Agent 实例
2. WHEN Agent 启动成功, THE Agent SHALL 在 5 秒内完成 WebSocket 连接并向 Server 发送 `auth` 消息
3. WHEN Server 收到 `auth` 消息并验证 Agent Token, THE Server SHALL 响应 `{"status": "authenticated"}` 消息
4. WHEN 认证通过后 Agent 进入心跳循环, THE Agent SHALL 每 30 秒（可配置）发送一次 `heartbeat` 消息
5. WHEN Server 收到 `heartbeat`, THE Server SHALL 更新该设备的在线状态和最后一次心跳时间
6. WHEN 通过 Server API 创建一个终端会话, THE Server SHALL 向 Agent 发送 `create_pty` 消息并在 10 秒内收到 `pty_ready` 响应
7. WHEN 终端会话建立后, THE 双向数据传输（浏览器输入 -> Server -> Agent PTY -> Agent -> Server -> 浏览器）的延迟 SHALL 不超过 200ms（在同机房网络下）
8. WHEN Server 主动断开 Agent 连接, THE Agent SHALL 在检测到连接断开后启动指数退避重连（1s -> 2s -> 4s -> 8s -> 16s -> 30s max）
9. WHEN Agent 在 5 次重连尝试后仍无法连接 Server, THE Agent SHALL 记录 error 级别日志并进入待机状态，每分钟尝试一次

### Requirement 4: Agent 端到端 (E2E) 测试

**User Story:** AS QA 工程师, I WANT 模拟用户在浏览器中的完整操作流程, SO THAT 验证从用户前端到边缘设备的全链路功能正常。

#### Acceptance Criteria

1. WHEN 执行 E2E 测试, THE 测试框架 SHALL 启动 Server、Agent（容器化）和 Playwright/Cypress 浏览器测试
2. WHEN 用户通过浏览器登录 Dashboard, THE 测试 SHALL 验证页面跳转到设备列表页且显示在线设备
3. WHEN 用户点击设备的 Terminal 按钮, THE 测试 SHALL 验证 xterm.js 终端在 15 秒内加载完成并显示 shell prompt
4. WHEN 在终端中输入 `echo hello`, THE 测试 SHALL 验证终端输出中包含 `hello` 字符串
5. WHEN 用户通过 Files 页面浏览目录, THE 测试 SHALL 验证文件和文件夹列表正确显示，且目录排在文件前面
6. WHEN 用户通过文件编辑器修改一个文本文件并保存, THE 测试 SHALL 验证修改已成功保存（通过重新加载文件内容验证）
7. WHEN 用户在 Dashboard 中点击 Logout, THE 测试 SHALL 验证页面跳转到登录页且本地 token 已清除

### Requirement 5: Agent 安全测试

**User Story:** AS 安全工程师, I WANT 验证 Agent 和 Server 之间的通信安全以及命令过滤机制, SO THAT 防止未授权访问和危险操作。

#### Acceptance Criteria

1. WHEN Agent 与 Server 通信, THE WebSocket 连接 SHALL 使用 `wss://` 协议（TLS 加密）
2. WHEN Agent 发送 `auth` 消息, THE Server SHALL 验证 Agent Token 的哈希值（数据库中存储哈希而非明文）
3. WHEN 用户在终端中执行危险命令（如 `rm -rf /`、`dd if=...`、fork bomb）, THE Server 命令过滤器 SHALL 拒绝执行并返回错误提示
4. WHEN 用户在终端中执行敏感命令（如 `sudo`、`reboot`、`systemctl restart`）, THE Server SHALL 要求二次确认（通过前端弹窗）
5. WHEN 尝试访问受保护路径（如 `/etc/shadow`、`/proc/`、`/sys/`）, THE Agent 的 `protectedPaths` 过滤器 SHALL 拒绝该操作并返回 `access denied` 错误
6. WHEN 尝试通过路径遍历（`../../../etc/passwd`）访问系统文件, THE Agent 的 `sanitizedPath` 函数 SHALL 拒绝该请求
7. WHEN Agent 同时创建超过 3 个 PTY 会话, THE Agent 的 semaphore 限制 SHALL 拒绝第 4 个会话创建请求
8. WHEN Server 检测到 Agent 连接已认证但超过 90 秒未收到心跳, THE Server SHALL 将设备状态标记为 `heartbeat_miss`
9. WHEN Server 检测到 Agent 连接已认证但超过 300 秒未收到心跳, THE Server SHALL 将设备状态标记为 `offline` 并移除在线映射

### Requirement 6: Agent 部署流程（install.sh + systemd）

**User Story:** AS 现场运维人员, I WANT 通过一个安装脚本将 Agent 部署到边缘设备并配置为开机自启动服务, SO THAT 设备重启后 Agent 自动恢复运行。

#### Acceptance Criteria

1. WHEN 执行 `install.sh`, THE 脚本 SHALL 自动检测当前设备的架构（aarch64 -> arm64, x86_64 -> amd64, i686 -> i386）并下载对应的二进制文件
2. WHEN 下载完成, THE 脚本 SHALL 验证二进制文件的 SHA256 校验和是否与 `checksums.txt` 一致
3. WHEN 校验通过, THE 脚本 SHALL 将二进制文件复制到 `/usr/local/bin/robot-agent` 并设置可执行权限
4. WHEN 配置文件 `agent.env` 不存在, THE 脚本 SHALL 提示用户输入 SERVER_URL、AGENT_TOKEN 和 DEVICE_ID
5. WHEN 配置文件已存在, THE 脚本 SHALL 询问用户是否覆盖现有配置
6. WHEN 安装完成, THE 脚本 SHALL 启用并启动 `robot-agent` systemd service
7. WHEN Agent 服务启动后, THE 脚本 SHALL 等待 10 秒并检查 `systemctl status robot-agent`，确认服务运行状态为 `active (running)`
8. IF Agent 服务在 10 秒后仍未 active, THE 脚本 SHALL 输出 `journalctl -u robot-agent --no-pager -n 20` 的最后 20 行日志以供排查
9. WHEN Agent 服务运行正常, THE 安装脚本 SHALL 输出一条成功消息：`Agent installed and running successfully on device {DEVICE_ID}`

### Requirement 7: Agent 监控与健康检查

**User Story:** AS 平台运维人员, I WANT 通过 Dashboard 实时查看所有 Agent 的连接状态、心跳延迟和资源使用情况, SO THAT 及时发现异常设备。

#### Acceptance Criteria

1. WHILE Agent 处于 `StateAuthenticated` 状态, THE Agent SHALL 每 30 秒（可配置）向 Server 发送一次心跳消息
2. WHEN Server 收到心跳, THE Server SHALL 记录心跳时间戳并在 Dashboard 的 Devices 页面更新设备状态为 `online`
3. WHEN Agent 连接断开或心跳超时, THE Server SHALL 在 Dashboard 中将设备状态更新为 `offline`
4. WHEN Server 检测到 Agent 版本与当前最新版本不一致, THE Server SHALL 在 Dashboard 中标记该设备需要升级
5. WHILE Agent 运行, THE Agent SHALL 记录日志到 syslog 或 `/var/log/robot-agent/agent.log`（可配置），日志级别包括 info、warn、error、debug
6. WHEN Agent 检测到连接断开, THE Agent SHALL 在日志中记录断开原因（网络错误、Server 关闭、认证失败等）和重连尝试次数

### Requirement 8: Agent 版本升级与回滚

**User Story:** AS 发布工程师, I WANT 分阶段（Canary -> 小批量 -> 全量）推送 Agent 新版本, SO THAT 在出现问题时能够最小化影响范围并快速回滚。

#### Acceptance Criteria

1. WHEN 启动灰度发布, THE 发布系统 SHALL 先向 5% 的设备推送新版本 Agent
2. WHEN 灰度阶段持续 24 小时且无崩溃报告, THE 发布系统 SHALL 自动扩展到 25% 的设备
3. WHEN 所有灰度阶段的设备都运行正常超过 24 小时, THE 发布系统 SHALL 扩展到 100% 的设备
4. IF 在灰度阶段有超过 2% 的设备报告 Agent 崩溃, THE 发布系统 SHALL 暂停推送并向运维团队发送告警
5. WHEN 运维团队触发回滚操作, THE 回滚流程 SHALL 向所有已升级的设备下发旧版本 Agent 的安装指令
6. WHEN Agent 收到降级指令, THE Agent SHALL 下载旧版本二进制文件、替换当前文件并重启服务（通过 systemd restart）
7. IF Agent 在升级过程中下载失败或校验和不匹配, THE Agent SHALL 保留旧版本二进制文件并回滚到旧版本继续运行
8. WHEN 回滚完成, THE 发布系统 SHALL 记录回滚操作详情（触发原因、影响设备数量、回滚时间）到审计日志

### Requirement 9: Agent 卸载流程

**User Story:** AS 现场运维人员, I WANT 通过一个卸载脚本从边缘设备完全移除 Agent, SO THAT 不再需要远程维护的设备可以彻底清理。

#### Acceptance Criteria

1. WHEN 执行 `uninstall.sh`, THE 脚本 SHALL 首先停止 `robot-agent` systemd service
2. WHEN 服务已停止, THE 脚本 SHALL 删除 systemd 服务文件 `/etc/systemd/system/robot-agent.service`
3. WHEN 服务文件已删除, THE 脚本 SHALL 执行 `systemctl daemon-reload` 以确保 systemd 配置重载
4. WHEN systemd 已重载, THE 脚本 SHALL 删除 Agent 二进制文件 `/usr/local/bin/robot-agent`
5. WHEN 卸载完成, THE 脚本 SHALL 询问用户是否保留 `/etc/robot-agent/` 目录（含 agent.env 配置文件）
6. IF 用户选择清理所有配置, THE 脚本 SHALL 递归删除 `/etc/robot-agent/` 目录及其所有内容
