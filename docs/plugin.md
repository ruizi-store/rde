# RDE 插件开发指南

## 目录

- [架构概述](#架构概述)
- [快速开始](#快速开始)
- [插件结构](#插件结构)
- [Manifest 配置](#manifest-配置)
- [插件 SDK](#插件-sdk)
- [模块开发](#模块开发)
- [前端集成](#前端集成)
- [数据库](#数据库)
- [API 路由](#api-路由)
- [WebSocket](#websocket)
- [生命周期](#生命周期)
- [调试与测试](#调试与测试)
- [打包与分发](#打包与分发)
- [完整示例](#完整示例)
- [最佳实践](#最佳实践)
- [FAQ](#faq)

---

## 架构概述

RDE 采用 **多进程 + Unix Socket HTTP** 插件架构。每个插件是一个独立的可执行文件，作为子进程被主进程启动，双方通过 Unix Domain Socket 上的标准 HTTP 协议通信。

```
┌─────────────────────────────────────────────────────┐
│                    RDE 主进程                         │
│                                                       │
│   Gin Router ──▶ /api/v1/{module}   内置模块直接处理   │
│       │                                               │
│       └──▶ /api/v1/{plugin-route}   NoRoute 回调      │
│                  │                                    │
│           Plugin Manager                              │
│           ├── 路由前缀匹配                              │
│           └── httputil.ReverseProxy                   │
│                  │                                    │
└──────────────────┼────────────────────────────────────┘
                   │ Unix Socket
        ┌──────────┼──────────┐
        │          │          │
   /run/rde/    /run/rde/   /run/rde/
   ai.sock     vm.sock    my-plugin.sock
        │          │          │
   ┌────┴───┐ ┌───┴────┐ ┌───┴────────┐
   │ rde-ai │ │ rde-vm │ │ my-plugin  │
   │  进程   │ │  进程   │ │   进程      │
   │ Gin    │ │ Gin    │ │ Gin        │
   │ HTTP   │ │ HTTP   │ │ HTTP       │
   └────────┘ └────────┘ └────────────┘
```

**核心特性**：
- **进程隔离** — 插件崩溃不影响主进程和其他插件
- **热插拔** — 将插件目录放入 `/var/lib/rde/plugins/` 即自动发现并启动
- **自动重启** — 进程异常退出后 5 秒自动重启
- **健康检查** — 主进程每 30 秒轮询 `/health` 端点
- **认证透传** — 主进程验证 JWT 后，请求透明转发到插件

---

## 快速开始

### 1. 创建项目骨架

```bash
mkdir -p my-plugin/{cmd,frontend}
cd my-plugin
```

### 2. 初始化 Go Module

```bash
go mod init github.com/yourname/rde-plugin-myplugin
```

添加 SDK 依赖：

```bash
# 如果 SDK 已发布到 Git
go get github.com/ruizi-store/rde-plugin-common/go@latest

# 本地开发时使用 replace
# 在 go.mod 末尾添加：
# replace github.com/ruizi-store/rde-plugin-common/go => ../common/go
```

### 3. 编写入口文件

**cmd/main.go**：

```go
package main

import (
    "fmt"
    "os"

    myplugin "github.com/yourname/rde-plugin-myplugin"
    "github.com/ruizi-store/rde-plugin-common/go/sdk"
)

func main() {
    p := sdk.New("my-plugin", "1.0.0")

    // 按需启用数据库
    // p.EnableDB()

    // 注册业务模块
    p.AddModule(myplugin.New())

    if err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
        os.Exit(1)
    }
}
```

### 4. 实现业务模块

**module.go**：

```go
package myplugin

import (
    "github.com/gin-gonic/gin"
    "github.com/ruizi-store/rde-plugin-common/go/sdk"
    "go.uber.org/zap"
)

type Module struct {
    logger *zap.Logger
}

func New() *Module {
    return &Module{}
}

func (m *Module) ID() string { return "myplugin" }

func (m *Module) Init(ctx *sdk.PluginContext) error {
    m.logger = ctx.Logger
    return nil
}

func (m *Module) Start() error {
    m.logger.Info("my-plugin started")
    return nil
}

func (m *Module) Stop() error {
    m.logger.Info("my-plugin stopped")
    return nil
}

func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
    api.GET("/myplugin/hello", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello from my-plugin!"})
    })
}
```

### 5. 编写 manifest.json

```json
{
    "id": "my-plugin",
    "name": "我的插件",
    "version": "1.0.0",
    "description": "示例插件",
    "binary": "my-plugin",
    "min_rde_version": "0.2.0",
    "routes": ["/myplugin/*"]
}
```

### 6. 编译与安装

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o my-plugin ./cmd/

# 部署到 RDE
sudo mkdir -p /var/lib/rde/plugins/my-plugin
sudo cp my-plugin /var/lib/rde/plugins/my-plugin/
sudo cp manifest.json /var/lib/rde/plugins/my-plugin/
# 插件会被自动发现并启动（无需重启 RDE）
```

### 7. 验证

```bash
# 插件列表
curl http://localhost:3080/api/v1/plugins

# 调用插件 API
curl http://localhost:3080/api/v1/myplugin/hello
# => {"message":"Hello from my-plugin!"}
```

---

## 插件结构

### 目录布局

```
my-plugin/
├── cmd/
│   └── main.go              # 入口：创建 Plugin、注册模块、调用 Run()
├── module.go                # 业务模块：实现 PluginModule 接口
├── handler.go               # HTTP 路由处理器
├── service.go               # 业务逻辑（可选）
├── model.go                 # 数据模型（可选）
├── go.mod
├── go.sum
├── manifest.json            # 插件声明文件
└── frontend/                # 前端代码（可选）
    ├── src/
    ├── package.json
    └── dist/                # 前端构建产物
```

### 安装后目录

插件安装在 `/var/lib/rde/plugins/{plugin-id}/`：

```
/var/lib/rde/plugins/my-plugin/
├── my-plugin                # 可执行二进制
├── manifest.json            # 插件声明
└── frontend/                # 前端静态文件（可选）
    └── dist/
```

运行时数据存储在 `/var/lib/rde/plugin-data/{plugin-id}/`：

```
/var/lib/rde/plugin-data/my-plugin/
├── plugin.db                # SQLite 数据库（如果启用了 EnableDB）
└── ...                      # 其他运行时数据
```

---

## Manifest 配置

每个插件必须包含 `manifest.json`，定义插件的身份、路由和前端应用。

### 完整字段说明

```jsonc
{
    // [必填] 插件唯一 ID，用于路由匹配、目录命名、socket 命名
    // 规范：小写字母 + 短横线，如 "my-plugin"
    "id": "my-plugin",

    // [必填] 显示名称
    "name": "我的插件",

    // [推荐] 语义化版本
    "version": "1.0.0",

    // [可选] 插件描述
    "description": "这是一个示例插件",

    // [可选] 可执行文件名，默认 "plugin"
    // 相对于插件安装目录
    "binary": "my-plugin",

    // [可选] 要求的最低 RDE 版本
    "min_rde_version": "0.2.0",

    // [必填] API 路由前缀列表
    // 主进程会将匹配 /api/v1{route} 的请求转发到本插件
    // 支持通配符 /*，如 "/myplugin/*" 匹配 /api/v1/myplugin/ 下所有路径
    "routes": ["/myplugin/*"],

    // [可选] 不需要 JWT 认证的路由
    // 常用于 webhook 回调等公开接口
    "public_routes": ["/myplugin/webhook"],

    // [可选] 前端桌面应用定义
    // 定义后用户可在 RDE 桌面看到应用图标
    "apps": [
        {
            "id": "my-app",                    // 应用唯一 ID
            "name": "我的应用",                  // 显示名称
            "icon": "/icons/my-app.svg",       // 图标路径（相对于 www）
            "frontend_route": "/app/myplugin/", // 前端访问路径
            "category": "tools",               // 分类: tools/system/media/other
            "default_width": 900,              // 默认窗口宽度 (px)
            "default_height": 650,             // 默认窗口高度 (px)
            "min_width": 600,                  // 最小窗口宽度
            "min_height": 450,                 // 最小窗口高度
            "singleton": true,                 // 是否单例（只能打开一个窗口）
            "permissions": ["files:read"]      // 声明需要的权限
        }
    ]
}
```

### 路由匹配规则

主进程收到 HTTP 请求时，按以下逻辑分派：

1. **内置模块** — Gin 路由表精确匹配（如 `/api/v1/files/*`）
2. **插件 API** — 内置路由未匹配时，遍历所有插件的 `routes` 前缀做匹配
3. **插件前端** — `/app/*` 路径匹配插件 `apps[].frontend_route`

假设你的 manifest 声明了 `"routes": ["/myplugin/*"]`，则：

| 请求路径 | 处理方 |
|---|---|
| `GET /api/v1/myplugin/hello` | → 转发到你的插件 |
| `GET /api/v1/myplugin/config` | → 转发到你的插件 |
| `GET /api/v1/files/list` | → RDE 内置文件模块 |
| `GET /app/myplugin/index.html` | → 转发到你的插件前端路由 |

---

## 插件 SDK

SDK 封装了 Unix Socket 服务器、日志、数据库、信号处理等通用逻辑。

### 核心类型

```go
import "github.com/ruizi-store/rde-plugin-common/go/sdk"
```

#### Plugin

```go
// 创建插件实例
p := sdk.New("my-plugin", "1.0.0")

// 配置
p.EnableDB()                          // 启用 SQLite
p.AddModule(myModule)                 // 注册业务模块
p.UseAPIMiddleware(authMiddleware)    // 添加 API 中间件
p.ServeFrontend("/app/xxx", "./dist") // 注册磁盘前端目录
p.ServeFrontendFS("/app/xxx", embedFS)// 注册嵌入式前端

// 启动（阻塞，直到收到 SIGINT/SIGTERM）
p.Run()
```

#### PluginModule 接口

每个插件至少实现一个 `PluginModule`：

```go
type PluginModule interface {
    // ID 模块唯一标识（用于日志 Named logger）
    ID() string

    // Init 初始化：接收 PluginContext，可在此做数据库迁移等
    Init(ctx *PluginContext) error

    // Start 启动后台任务（如定时器、worker 等）
    Start() error

    // Stop 优雅停止（释放资源、关闭连接）
    Stop() error

    // RegisterRoutes 注册 HTTP 路由到 /api/v1 路由组
    RegisterRoutes(group *gin.RouterGroup)
}
```

#### PluginContext

SDK 在初始化时创建，传递给每个模块的 `Init()` 方法：

```go
type PluginContext struct {
    Logger  *zap.Logger  // 带模块名的 Named logger
    DataDir string       // 插件数据目录，如 /var/lib/rde/plugin-data/my-plugin
    BaseDir string       // RDE 安装根目录
    DB      *gorm.DB     // SQLite 数据库实例（需 EnableDB）
    Debug   bool         // 调试模式
}
```

### 命令行参数

SDK 自动解析以下参数（由主进程在启动插件时传入）：

| 参数 | 说明 | 默认值 |
|---|---|---|
| `--socket` | Unix Socket 路径 | `/var/run/rde/plugins/plugin.sock` |
| `--data-dir` | 插件数据存储目录 | `/var/lib/rde/plugin-data/plugin` |
| `--base-dir` | RDE 安装根目录 | `/opt/rde` |
| `--debug` | 调试模式 | `false` |

> **注意**：这些参数由主进程自动传入，开发者无需手动指定。调试时可手动运行：
> ```bash
> ./my-plugin --socket /tmp/my-plugin.sock --data-dir /tmp/my-plugin-data --debug
> ```

---

## 模块开发

### 典型模块结构

```go
package myplugin

import (
    "github.com/gin-gonic/gin"
    "github.com/ruizi-store/rde-plugin-common/go/sdk"
    "go.uber.org/zap"
    "gorm.io/gorm"
)

// Model 数据模型
type Config struct {
    gorm.Model
    Key   string `gorm:"uniqueIndex" json:"key"`
    Value string `json:"value"`
}

// Module 业务模块
type Module struct {
    logger *zap.Logger
    db     *gorm.DB
}

func New() *Module {
    return &Module{}
}

func (m *Module) ID() string { return "myplugin" }

func (m *Module) Init(ctx *sdk.PluginContext) error {
    m.logger = ctx.Logger
    m.db = ctx.DB

    // 自动迁移数据表
    if m.db != nil {
        if err := m.db.AutoMigrate(&Config{}); err != nil {
            return err
        }
    }
    return nil
}

func (m *Module) Start() error {
    // 启动后台任务、定时器等
    return nil
}

func (m *Module) Stop() error {
    // 清理资源
    return nil
}

func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
    g := api.Group("/myplugin")
    {
        g.GET("/config", m.getConfig)
        g.PUT("/config", m.updateConfig)
        g.GET("/status", m.getStatus)
    }
}

// --- HTTP Handlers ---

func (m *Module) getConfig(c *gin.Context) {
    var configs []Config
    m.db.Find(&configs)
    c.JSON(200, configs)
}

func (m *Module) updateConfig(c *gin.Context) {
    var req Config
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    m.db.Where("key = ?", req.Key).Assign(Config{Value: req.Value}).FirstOrCreate(&req)
    c.JSON(200, req)
}

func (m *Module) getStatus(c *gin.Context) {
    c.JSON(200, gin.H{"status": "running"})
}
```

### 多模块注册

一个插件可以注册多个模块：

```go
func main() {
    p := sdk.New("my-plugin", "1.0.0")
    p.EnableDB()

    p.AddModule(myplugin.NewCoreModule())
    p.AddModule(myplugin.NewSchedulerModule())
    p.AddModule(myplugin.NewNotificationModule())

    p.Run()
}
```

模块按注册顺序初始化和启动，逆序停止。

---

## 前端集成

插件可以提供独立的前端界面，在 RDE 桌面上显示为一个应用窗口。

### 方式一：嵌入二进制（embed.FS）

前端构建产物嵌入到 Go 二进制中，无需额外分发文件。

**embed.go**：

```go
package myplugin

import "embed"

//go:embed all:frontend/dist
var FrontendDist embed.FS
```

**cmd/main.go**：

```go
func main() {
    p := sdk.New("my-plugin", "1.0.0")
    p.AddModule(myplugin.New())

    // 嵌入式前端
    frontendFS, _ := fs.Sub(myplugin.FrontendDist, "frontend/dist")
    p.ServeFrontendFS("/app/myplugin", frontendFS)

    p.Run()
}
```

**优点**：单文件部署，不用管前端文件路径
**缺点**：增加二进制体积

### 方式二：磁盘目录

前端文件放在插件目录下的 `frontend/` 中。

**cmd/main.go**：

```go
func main() {
    p := sdk.New("my-plugin", "1.0.0")
    p.AddModule(myplugin.New())

    // 基于磁盘目录的前端
    exe, _ := os.Executable()
    execDir := filepath.Dir(exe)
    p.ServeFrontend("/app/myplugin", filepath.Join(execDir, "frontend", "dist"))

    p.Run()
}
```

**优点**：前端可独立更新
**缺点**：需要确保文件目录结构正确

### 前端路由说明

SDK 内置 SPA 路由支持：
- 请求静态文件（JS/CSS/图片等），直接返回文件
- 请求路径不匹配任何文件时，回退到 `index.html`（支持前端路由）
- `assets/` 和 `immutable/` 目录下的文件自动设置长期缓存头
- `*.html` 文件设置 `no-cache`

### 前端调用 API

前端通过标准 HTTP 请求访问插件 API。主进程的 JWT 认证对插件请求透明——前端只需携带 RDE 登录时的 token：

```typescript
// 前端代码示例
const resp = await fetch("/api/v1/myplugin/config", {
  headers: {
    Authorization: `Bearer ${token}`,
  },
});
const data = await resp.json();
```

---

## 数据库

调用 `p.EnableDB()` 后，SDK 会在 `{dataDir}/plugin.db` 创建 SQLite 数据库，使用 WAL 模式。

### 数据库迁移

在 `Init()` 方法中使用 GORM 的 `AutoMigrate`：

```go
func (m *Module) Init(ctx *sdk.PluginContext) error {
    m.db = ctx.DB
    return m.db.AutoMigrate(&MyModel{}, &AnotherModel{})
}
```

### 使用说明

- 数据库基于 `glebarez/sqlite`（纯 Go 实现，无需 CGO）
- 默认开启 WAL 模式，支持并发读
- `ctx.DB` 是标准的 `*gorm.DB` 实例，用法与 GORM 完全相同
- 数据库文件路径：`/var/lib/rde/plugin-data/{plugin-id}/plugin.db`

### 不需要数据库？

如果你的插件不需要持久化数据（例如只是代理外部 API），不调用 `EnableDB()` 即可，此时 `ctx.DB` 为 `nil`。

---

## API 路由

### 路由注册

在 `RegisterRoutes` 中注册路由，路由组已挂载在 `/api/v1` 下：

```go
func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
    // 最终路径: /api/v1/myplugin/xxx
    g := api.Group("/myplugin")
    {
        g.GET("/items", m.listItems)
        g.POST("/items", m.createItem)
        g.GET("/items/:id", m.getItem)
        g.PUT("/items/:id", m.updateItem)
        g.DELETE("/items/:id", m.deleteItem)
    }
}
```

### 认证

主进程在转发请求前已完成 JWT 认证。插件默认收到的请求都是已认证的。

如需声明无需认证的路由（如 webhook），在 `manifest.json` 的 `public_routes` 中声明：

```json
{
    "routes": ["/myplugin/*"],
    "public_routes": ["/myplugin/webhook"]
}
```

### 中间件

可通过 `UseAPIMiddleware` 添加自定义中间件：

```go
p := sdk.New("my-plugin", "1.0.0")
p.UseAPIMiddleware(func(c *gin.Context) {
    // 自定义逻辑（如 License 校验）
    c.Next()
})
```

### SSE（Server-Sent Events）

SDK 的反向代理已设置 `FlushInterval: -1`，天然支持 SSE 流式响应：

```go
func (m *Module) streamEvents(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    flusher := c.Writer.(http.Flusher)
    for {
        select {
        case event := <-m.eventCh:
            fmt.Fprintf(c.Writer, "data: %s\n\n", event)
            flusher.Flush()
        case <-c.Request.Context().Done():
            return
        }
    }
}
```

---

## WebSocket

SDK 支持 WebSocket 连接的透明代理。主进程检测到 `Upgrade: websocket` 头后，会在 TCP 层面建立客户端与插件之间的双向桥接。

在插件中使用 WebSocket（以 `gorilla/websocket` 为例）：

```go
import "github.com/gorilla/websocket"

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
    api.GET("/myplugin/ws", m.handleWS)
}

func (m *Module) handleWS(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            break
        }
        conn.WriteMessage(websocket.TextMessage, msg) // echo
    }
}
```

---

## 生命周期

### 启动流程

```
主进程启动
  └─▶ PluginManager.Discover()
        └─▶ 扫描 /var/lib/rde/plugins/*/manifest.json
  └─▶ PluginManager.StartAll()
        └─▶ 对每个非禁用插件:
              1. exec.Command(binaryPath, --socket, --data-dir, --base-dir)
              2. 等待 Unix Socket 就绪（最多 10 秒）
              3. 发送 GET /health 健康检查
              4. 标记为 StateRunning
              5. 启动进程监控 goroutine
```

### 运行时

| 机制 | 说明 |
|---|---|
| **健康检查** | 每 30 秒 `GET /health`，自动由 SDK 处理 |
| **自动重启** | 进程退出后 5 秒自动重启（除非被禁用） |
| **热加载** | fsnotify 监听插件目录，新插件目录出现 → 2 秒防抖 → 自动加载 |
| **热重载** | manifest.json 或二进制文件变更 → 停止旧进程 → 重新加载启动 |
| **热卸载** | 删除插件目录 → 自动停止进程并移除 |

### 停止流程

```
收到 SIGINT/SIGTERM
  └─▶ Plugin.Run() 捕获信号
        1. HTTP Server 优雅关闭（5 秒超时）
        2. 逆序调用各模块 Stop()
        3. 进程退出
```

### 状态机

```
StateStopped ──▶ StateStarting ──▶ StateRunning
                      │                  │
                      ▼                  ▼ (进程崩溃)
                  StateError ◀───────────┘
                      │
                      ▼ (5 秒后自动重启)
                  StateStarting
```

---

## 调试与测试

### 本地独立运行

无需启动 RDE 主进程，直接运行插件：

```bash
# 编译
CGO_ENABLED=0 go build -o my-plugin ./cmd/

# 独立运行（指定本地 socket 和数据目录）
./my-plugin \
  --socket /tmp/my-plugin.sock \
  --data-dir /tmp/my-plugin-data \
  --debug

# 另一个终端测试
curl --unix-socket /tmp/my-plugin.sock http://localhost/health
curl --unix-socket /tmp/my-plugin.sock http://localhost/api/v1/myplugin/hello
```

### 接入 RDE 调试

将编译产物复制到插件目录：

```bash
sudo cp my-plugin manifest.json /var/lib/rde/plugins/my-plugin/
# 热加载自动生效，无需重启 RDE
```

查看主进程日志确认加载：

```bash
journalctl -u rde -f | grep plugin
```

### 启用/禁用插件

通过 API：

```bash
# 禁用
curl -X POST http://localhost:3080/api/v1/plugins/my-plugin/disable

# 启用
curl -X POST http://localhost:3080/api/v1/plugins/my-plugin/enable

# 查看状态
curl http://localhost:3080/api/v1/plugins
```

### 单元测试

```go
func TestMyHandler(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // 创建测试数据库
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    db.AutoMigrate(&MyModel{})

    // 初始化模块
    m := New()
    m.Init(&sdk.PluginContext{
        Logger: zap.NewNop(),
        DB:     db,
    })

    // 注册路由
    router := gin.New()
    api := router.Group("/api/v1")
    m.RegisterRoutes(api)

    // 执行测试请求
    w := httptest.NewRecorder()
    req, _ := http.NewRequest("GET", "/api/v1/myplugin/status", nil)
    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)
}
```

---

## 打包与分发

### 目录结构要求

最终分发的插件目录至少包含：

```
my-plugin/
├── my-plugin            # 可执行二进制（名称需与 manifest.binary 一致）
├── manifest.json        # 插件声明
└── frontend/            # 前端文件（可选，若用 embed 则不需要）
    └── dist/
```

### 编译建议

```bash
# 推荐编译参数
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" -o my-plugin ./cmd/

# ARM64 交叉编译
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -ldflags="-s -w" -o my-plugin ./cmd/
```

| 参数 | 说明 |
|---|---|
| `CGO_ENABLED=0` | 纯静态编译，无 C 依赖，体积更小 |
| `-ldflags="-s -w"` | 去除符号表和调试信息 |

### 制作 DEB 包（可选）

参考 `plugins/cloud-backup/debian/` 目录的结构，创建标准 Debian 包。

### 安装

```bash
# 方式一：直接复制
sudo cp -r my-plugin/ /var/lib/rde/plugins/

# 方式二：DEB 包
sudo dpkg -i rde-plugin-myplugin_1.0.0_amd64.deb
```

---

## 完整示例

以下是一个带数据库和前端的完整「待办事项」插件示例。

### manifest.json

```json
{
    "id": "todo",
    "name": "待办事项",
    "version": "1.0.0",
    "description": "简单的待办事项管理",
    "binary": "rde-todo",
    "routes": ["/todo/*"],
    "apps": [
        {
            "id": "todo",
            "name": "待办事项",
            "icon": "/icons/todo.svg",
            "frontend_route": "/app/todo/",
            "category": "tools",
            "default_width": 600,
            "default_height": 500,
            "singleton": true
        }
    ]
}
```

### model.go

```go
package todo

import "gorm.io/gorm"

type Todo struct {
    gorm.Model
    Title     string `json:"title"`
    Completed bool   `json:"completed"`
    Priority  int    `json:"priority"`
}
```

### module.go

```go
package todo

import (
    "github.com/gin-gonic/gin"
    "github.com/ruizi-store/rde-plugin-common/go/sdk"
    "go.uber.org/zap"
    "gorm.io/gorm"
)

type Module struct {
    logger *zap.Logger
    db     *gorm.DB
}

func New() *Module { return &Module{} }

func (m *Module) ID() string { return "todo" }

func (m *Module) Init(ctx *sdk.PluginContext) error {
    m.logger = ctx.Logger
    m.db = ctx.DB
    return m.db.AutoMigrate(&Todo{})
}

func (m *Module) Start() error { return nil }
func (m *Module) Stop() error  { return nil }

func (m *Module) RegisterRoutes(api *gin.RouterGroup) {
    g := api.Group("/todo")
    g.GET("/items", m.list)
    g.POST("/items", m.create)
    g.PUT("/items/:id", m.update)
    g.DELETE("/items/:id", m.delete)
}

func (m *Module) list(c *gin.Context) {
    var items []Todo
    m.db.Order("priority desc, created_at desc").Find(&items)
    c.JSON(200, items)
}

func (m *Module) create(c *gin.Context) {
    var item Todo
    if err := c.ShouldBindJSON(&item); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    m.db.Create(&item)
    c.JSON(201, item)
}

func (m *Module) update(c *gin.Context) {
    id := c.Param("id")
    var item Todo
    if err := m.db.First(&item, id).Error; err != nil {
        c.JSON(404, gin.H{"error": "not found"})
        return
    }
    c.ShouldBindJSON(&item)
    m.db.Save(&item)
    c.JSON(200, item)
}

func (m *Module) delete(c *gin.Context) {
    m.db.Delete(&Todo{}, c.Param("id"))
    c.JSON(200, gin.H{"ok": true})
}
```

### cmd/main.go

```go
package main

import (
    "fmt"
    "os"

    todo "github.com/yourname/rde-plugin-todo"
    "github.com/ruizi-store/rde-plugin-common/go/sdk"
)

func main() {
    p := sdk.New("rde-todo", "1.0.0")
    p.EnableDB()
    p.AddModule(todo.New())

    if err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
        os.Exit(1)
    }
}
```

---

## 最佳实践

### 编码规范

- **模块化** — 不同功能拆分为独立的 `PluginModule`，职责单一
- **错误处理** — `Init()` 返回 error 时模块被跳过，确保非关键模块失败不影响整体
- **日志** — 使用 `ctx.Logger`（zap），不要用 `fmt.Println` 或 `log`
- **路由前缀** — 使用与插件 ID 一致的前缀（如 `id: "todo"` → `/todo/*`）

### 性能

- **CGO_ENABLED=0** — 使用纯 Go SQLite 驱动（`glebarez/sqlite`），避免 CGO 增加二进制体积
- **连接复用** — SDK 的反向代理已配置 `MaxIdleConnsPerHost: 10`
- **WAL 模式** — 数据库默认使用 WAL，支持读写并发

### 安全

- **不信任输入** — 路由参数、请求体都需要校验
- **路径遍历防护** — SDK 前端服务已内置路径检查
- **权限声明** — 在 manifest 的 `permissions` 中声明需要的权限

### 部署

- **单文件优先** — 使用 `embed.FS` 嵌入前端，简化部署
- **版本兼容** — 设置 `min_rde_version` 防止在旧版 RDE 上运行
- **优雅停止** — 在 `Stop()` 中清理后台 goroutine、关闭连接

---

## FAQ

### 插件和内置模块有什么区别？

| 维度 | 内置模块 | 插件 |
|---|---|---|
| 加载方式 | 编译时链接，同进程 | 独立可执行文件，子进程 |
| 通信 | 直接函数调用 | Unix Socket HTTP 代理 |
| 热插拔 | 不支持 | 支持（自动发现/加载/卸载） |
| 隔离性 | 共享内存空间 | 进程级隔离 |
| 调试 | 需重新编译主程序 | 独立编译和运行 |

### 如何处理跨插件通信？

插件之间不直接通信。如需共享数据，推荐通过 RDE 主程序的 API 中转。

### 插件的 API 路由和内置模块冲突怎么办？

内置模块的 Gin 路由优先级更高。如果冲突，内置模块会先匹配，插件路由不会被使用。确保你的 `routes` 前缀不与内置模块重复（常见内置前缀：`/files`、`/users`、`/system`、`/docker`、`/terminal`、`/ssh`、`/samba` 等）。

### 能否在一个插件中注册多个 App？

可以。在 `manifest.json` 的 `apps` 数组中定义多个应用，每个有独立的 `id`、`frontend_route` 和窗口配置。

### 如何升级插件？

替换插件目录下的二进制和 manifest 文件即可，主进程的 fsnotify 会检测到变化并自动重载。或者停止旧版本、替换文件、重新启动。

### 支持哪些平台？

`CGO_ENABLED=0` 编译的插件支持 Linux amd64 和 arm64。与 RDE 主程序支持的平台一致。
