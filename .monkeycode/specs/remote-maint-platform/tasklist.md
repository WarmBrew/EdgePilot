# 远程运维平台实施任务清单

## Phase 1: 基础设施搭建 (1-2 天)

- [x] 1.1 项目结构初始化
  - 创建 `web/`, `server/`, `agent/` 目录
  - 配置 `.gitignore`, `.env.example`, `README.md`

- [x] 1.2 Docker Compose 编排
  - 编写 `docker-compose.yml` (postgres, redis, web, api)
  - 配置数据卷持久化
  - 编写健康检查配置

- [x] 1.3 环境变量与配置管理
  - 创建 `.env.example` 模板文件
  - 编写 Go 配置加载模块 (viper)
  - 编写前端环境变量注入 (Vite)

- [x] 1.4 CI/CD 基础配置
  - 编写 GitHub Actions Workflow
  - 配置自动构建和测试

## Phase 2: 数据库与数据模型 (2-3 天)

- [x] 2.1 PostgreSQL 数据库迁移
  - 编写 SQL 迁移文件 (tenants, users, devices, device_groups, terminal_sessions, audit_logs)
  - 实现 GORM 自动迁移
  - 添加索引 (tenant_id, status, created_at)

- [x] 2.2 GORM 数据模型定义
  - 定义 Tenant, User, Device, DeviceGroup 模型
  - 实现 UUID 主键自动生成
  - 实现软删除支持

- [x] 2.3 Redis 服务初始化
  - 编写 Redis 连接配置
  - 实现分布式锁工具
  - 实现限流计数器中间件

## Phase 3: 认证与授权 (3-4 天)

- [x] 3.1 用户注册与登录
  - 实现注册 API (邮箱校验 + 密码强度)
  - 实现登录 API (JWT 生成 + HttpOnly Cookie)
  - 实现密码 bcrypt 加密 (cost=12)

- [x] 3.2 JWT Token 管理
  - 实现 Access Token (15 分钟)
  - 实现 Refresh Token (7 天)
  - 实现令牌黑名单机制 (Redis)

- [x] 3.3 认证与授权中间件
  - 实现 AuthenticationMiddleware (JWT 验证)
  - 实现 AuthorizationMiddleware (RBAC 权限校验)
  - 实现租户隔离 (WHERE tenant_id = ?)

- [x] 3.4 输入验证中间件
  - 实现限流中间件 (RateLimiter)
  - 实现签名验证中间件 (HMAC-SHA256)
  - 实现输入参数校验器

## Phase 4: 设备管理 (3-4 天)

- [x] 4.1 设备注册与认证
  - 实现设备注册 API (返回 agent_token)
  - 实现设备状态管理 (online/offline/heartbeat_miss)

- [x] 4.2 设备 CRUD API
  - 实现设备列表 (分页/搜索/筛选)
  - 实现设备详情查询
  - 实现设备更新与删除
  - 实现批量操作

- [x] 4.3 设备分组管理
  - 实现分组 CRUD API
  - 实现设备分组分配
  - 实现分组统计

- [x] 4.4 心跳检测机制
  - 实现设备心跳 API (每 30 秒)
  - 实现离线检测定时任务
  - Redis 缓存设备状态映射

## Phase 5: WebSocket 终端网关 (5-7 天)

- [x] 5.1 WebSocket 网关
  - 实现 WebSocket 连接处理器
  - 实现设备 ID 到 WS 连接映射 (Redis)
  - 实现连接心跳监控

- [ ] 5.2 终端会话管理
  - 创建终端会话 (terminal_sessions)
  - 实现会话生命周期管理
  - 实现会话超时自动断开

- [ ] 5.3 终端转发服务
  - 实现终端消息协议 (input/output/resize/close)
  - 实现数据 Base64 编码
  - 实现并发会话限制 (单设备 3 个)

- [ ] 5.4 命令安全控制
  - 实现命令黑名单过滤
  - 实现只读模式白名单
  - 实现敏感命令二次确认机制

## Phase 6: 文件管理 (3-4 天)

- [ ] 6.1 文件操作 API
  - 实现文件列表 API (目录树)
  - 实现文件内容读取/写入
  - 实现文件上传/下载

- [ ] 6.2 文件安全控制
  - 实现路径遍历防护 (path.Clean)
  - 实现文件类型白名单
  - 实现文件大小限制 (100MB)
  - 实现符号链接防护

- [ ] 6.3 文件权限管理
  - 实现文件权限修改 API (chmod/chown)
  - 实现角色权限矩阵
  - 实现文件操作审计

## Phase 7: 审计日志与安全 (3-4 天)

- [ ] 7.1 审计日志框架
  - 实现审计日志中间件
  - 实现审计日志写入 (异步队列)
  - 实现日志存储 (PostgreSQL JSONB)

- [ ] 7.2 审计日志查询
  - 实现审计日志 API (分页/筛选/时间范围)
  - 实现日志导出 (CSV/JSON)
  - 实现日志脱敏显示

- [ ] 7.3 前端安全响应头
  - 实现 Helmet 等效 (XSS 防护)
  - 实现 CORS 配置
  - 实现 HTTPS 强制

## Phase 8: 前端框架与组件 (4-5 天)

- [ ] 8.1 Vue 3 项目脚手架
  - 配置 Vite + Vue 3 + Pinia
  - 配置 Vue Router 路由
  - 配置 Element Plus 组件库

- [ ] 8.2 布局与导航
  - 实现 MainLayout (侧边栏 + 顶栏)
  - 实现权限路由守卫
  - 实现菜单高亮/收起

- [ ] 8.3 可复用组件
  - 实现 WebSocket 客户端封装
  - 实现 API 请求封装 (Axios)
  - 实现 Token 自动刷新

- [ ] 8.4 页面实现
  - 登录页
  - 仪表盘页
  - 设备列表页
  - 设备分组管理
  - 审计日志页

## Phase 9: Agent 开发 (3-4 天)

- [ ] 9.1 Agent 基础架构
  - 实现配置加载
  - 实现 WebSocket 客户端 (重连机制)
  - 实现日志框架

- [ ] 9.2 PTY 终端模拟
  - 集成 creack/pty 库
  - 实现 PTY 生命周期管理
  - 实现终端输出转发

- [ ] 9.3 文件操作模块
  - 实现目录列表 (支持子目录)
  - 实现文件读写
  - 实现文件权限获取

- [ ] 9.4 Agent 部署
  - 实现交叉编译 (ARM64/AMD64)
  - 编写 systemd 服务安装脚本
  - 编写安装部署指南

## Phase 10: 测试与部署 (4-5 天)

- [ ] 10.1 单元试
  - 后端 Handler 测试
  - 后端 Service 测试
  - Agent 模块测试

- [ ] 10.2 集成测试
  - 终端端到端测试
  - 文件上传/下载测试
  - 设备管理流程测试

- [ ] 10.3 性能测试
  - WebSocket 并发测试 (100+ 设备)
  - 数据库压力测试
  - API 响应时间测试

- [ ] 10.4 部署文档
  - 编写生产部署指南
  - 编写运维手册
  - 编写 Agent 部署指南

---

**预期开发时间: 3-4 周**

**优先级:**
- P0: Phase 1 → 3 → 4 → 5 (设备管理和终端核心)
- P1: Phase 6 → 8 → 9 (文件管理和前端)
- P2: Phase 7 → 10 (审计和测试)
