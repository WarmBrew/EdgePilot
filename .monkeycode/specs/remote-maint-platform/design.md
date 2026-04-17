# 具身智能机器人远程运维平台 - 技术设计文档

## 1. 系统架构

### 1.1 架构模式
云端 SaaS + 边缘 Agent (多租户、多设备管理)

### 1.2 技术栈
| 层级 | 技术选型 | 理由 |
|------|---------|------|
| Web 控制台 | Vue 3 + Element Plus + Vite | 企业级 UI 组件，开箱即用 |
| API 服务 | Go + Gin | 轻量、高并发、类型安全 |
| WebSocket 网关 | Go + gorilla/websocket | 原生支持，与 API 服务统一部署 |
| 数据库 | PostgreSQL 15+ | 多租户、JSONB、全文搜索 |
| 缓存 | Redis 7+ | 会话、设备在线状态、消息队列 |
| 边缘 Agent | Go | 跨平台编译、低资源占用 |

### 1.3 架构图
```
用户浏览器 → Nginx (前端 + 反向代理)
              ↓
           API 网关 / 负载均衡
              ↓
    ┌─────────┼─────────┐
    ↓         ↓         ↓
 REST API   WS 网关   管理 API
 (Gin)     (gorilla)
    └───────┬─────────┘
            ↓
      业务逻辑层 (Service)
    ┌───────┼───────┐
    ↓               ↓
PostgreSQL        Redis
(主数据存储)      (缓存/会话/在线状态)
```

### 1.4 部署拓扑
- Docker Compose 编排 (PostgreSQL, Redis, Web, API)
- 数据卷持久化 (pg_data, redis_data, logs)
- 健康检查确保依赖就绪

## 2. 数据模型

### 2.1 核心表
- tenants: 多租户隔离
- users: 用户与角色 (RBAC)
- devices: 设备信息 (状态、在线、心跳)
- device_groups: 设备分组
- terminal_sessions: 终端会话
- audit_logs: 操作审计 (高频写入、JSONB)

### 2.2 索引设计
- idx_devices_tenant_status (租户 + 状态)
- idx_devices_group (分组查询)
- idx_audit_logs_tenant_time (租户 + 时间)

## 3. 安全架构

### 3.1 请求安全链
```
请求 → 限流中间件 → 签名验证 → JWT 认证 → RBAC 权限 → 输入验证 → 审计日志 → Handler
```

### 3.2 防护清单
| 威胁 | 防护手段 | 优先级 |
|------|---------|---------|
| 未授权访问 | JWT + HttpOnly Cookie | P0 |
| 水平越权 | 租户 ID 绑定 + 行级归属校验 | P0 |
| 垂直越权 | RBAC 权限校验中间件 | P0 |
| SQL 注入 | GORM 参数化查询 | P0 |
| 命令注入 | PTY 参数分离 + 黑白名单 | P0 |
| DDoS | IP 级别限流 + 动态封禁 | P0 |
| 暴力破解 | 失败次数限制 + IP 封禁 | P0 |
| 路径遍历 | path.Clean() + 前缀限制 | P0 |

## 4. 前端页面结构
```
Layout → Login → Dashboard → Devices → Terminal → Files → Audit → Settings
```

## 5. 实施计划
10 个 Phase，约 3-4 周开发时间

```
