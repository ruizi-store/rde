# rde-plugin-common

RDE 插件共享代码库，供各闭源插件模块引用。

## 结构

```
go/                     # Go 共享代码（Go Module）
├── go.mod              # module github.com/ruizi-store/rde-plugin-common/go
├── sdk/                # 插件 SDK（Unix Socket 服务器、模块注册、前端托管）
├── auth/               # License 验证
└── membership/         # 会员管理（云端同步、邮箱绑定、流量统计）

frontend/               # 前端共享代码（Svelte 5 + TailwindCSS 4）
├── ui/                 # 通用 UI 组件（Button, Modal, Tabs, ...）
├── components/         # 布局组件（AppShell, ToastContainer）
├── services/           # API 客户端
├── stores/             # 状态管理
├── styles/             # 全局样式
├── i18n/               # 国际化
├── utils/              # 工具函数
└── vite-shared.ts      # Vite 共享配置工厂
```

## 使用方式

### Go 模块

```go
import (
    "github.com/ruizi-store/rde-plugin-common/go/sdk"
    "github.com/ruizi-store/rde-plugin-common/go/auth"
    "github.com/ruizi-store/rde-plugin-common/go/membership"
)
```

### 前端（Git Submodule）

在插件项目中将此仓库作为 submodule 引入：

```bash
git submodule add git@github.com:ruizi-store/rde-plugin-common.git common
```

在 `vite.config.ts` 中配置 alias：

```ts
resolve: {
  alias: {
    '$shared': resolve(__dirname, 'common/frontend'),
  }
}
```
