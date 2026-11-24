# RDE 产品路线：带电脑的 Bot（小团队方向）

> 定位一句话：**自托管的 AI 同事——每个 Bot 有一台持久的电脑，团队成员在企微/飞书/钉钉/Telegram 里像给同事一样派活，需要人时回来找你。**
>
> 竞品参照：Grok Bot（商业标杆）、OpenTeam（开源、最接近）。RDE 的差异化底座：现成的 Web 桌面（窗口管理/文件/终端）直接复用为"Bot 的工作台，人可围观与接管"。

## 架构原则：控制面与 Bot 电脑分离

```
┌─────────────── RDE 容器（控制面，无 root 危险操作）───────────────┐
│  聊天 UI / IM 网关(企微/飞书/钉钉/TG) / 调度 / 记忆 / 审批 / 用量统计 │
└────────────┬───────────────────────────────────────────────────┘
             │ Docker API（创建/停止/串流）
┌────────────▼───────────── Bot 工作台（每个团队一台，独立容器）──────┐
│  Linux 桌面 + 浏览器(Chromium) + 终端 + /workspace 持久卷           │
│  Bot 的所有工具调用只发生在这里；可经 KasmVNC 串流到 RDE 桌面窗口    │
└─────────────────────────────────────────────────────────────────┘
```

- Bot 工具执行只在沙箱容器内；控制面进程不执行用户脚本（`ai/skills.go` 已加注释声明此边界）。
- 浏览器自动化优先走 DOM/CDP（Playwright MCP），截图式 computer-use 作为兜底（token 成本差一个量级）。
- 密钥不落盘明文：凭据由控制面在请求时注入沙箱（对齐 Grok Bot 的 secure form + OpenClaw 的 Credential Gateway 教训）。

## 小团队（而非个人）的关键设计

1. **渠道优先级**：企微自建应用 > 飞书 > 钉钉 > Telegram（国内小团队先能用）。现有 `gateway_wecom.go` 是起点，需补飞书/钉钉。
2. **多成员权限**：AllowedUsers 白名单已有（且已改为默认拒绝）；补角色（管理员/成员/只读）与按 Bot 的成员可见性。
3. **用量可观测**：token 用量按成员/按 Bot 统计并推送周报（费用是小团队最敏感的问题）。
4. **审批**：敏感操作（付款、发消息给外部、删数据）在 IM 中出"允许/拒绝"卡片。

## 分阶段计划

### P0 — 安全地基（已完成 ✅）
- `ai/gateway.go` checkSecurity 改为默认拒绝（白名单为空时拒绝所有消息）。
- `ai/skills.go` ExecuteScript 未知语言拒绝执行 + 解释器白名单 + 注释声明"控制面不得执行用户脚本"。

### P1 — 容器化交付（本次进行中 🚧）
- 模块开关：`RDE_MODULES` / `RDE_MODULES_DISABLED`（`core/bootstrap/bootstrap.go`，含单元测试）✅
- Dockerfile（多阶段：node20 + go1.25 + debian-slim）、`docker/entrypoint.sh`、`docker/docker-compose.yml`、`docker/README.md` ✅（构建验证中）
- Makefile `make docker` ✅
- 后续：CI 加 buildx 多架构构建并推镜像仓库；setup 向导在容器模式跳过 netplan/挂载/PAM/端口选择（`setup/service.go` 中按 `/proc/1/cgroup` 或 env 探测容器环境）。

### P2 — Bot 工作台（最小可用 ✅ 2026-11）
- 新模块 `modules/bot`：Bot 的 CRUD（名字/职责/模型/成员可见性），接管互斥锁。✅
- Bot 沙箱容器：复用 `docker` 模块 client；镜像 `rde/bot-desktop`（Debian trixie + Chromium + KasmVNC，
  KasmVNC deb 来自 `thirdparty/kasmvnc`，`make bot-desktop` 构建）。✅
- 三级可见性：状态（图标点亮）/ 预览（iframe 围观 KasmVNC 串流）/ 接管（互斥 + 30 分钟过期）。✅
- 工具调用走沙箱 exec（`ExecWithTimeout`：超时 300s 上限 / 输出 64KB 截断；控制面不执行用户脚本）。✅
- 网络拓扑：`rde-bot-net` bridge 网络，控制面容器与沙箱同网段（compose 已配置）。✅

### P3 — 技能与渠道（下一步）
- 技能文件化：`/var/lib/rde/skills/<name>/SKILL.md`（对齐 agent skills 生态格式），替代 `skills.go` 中硬编码的 22 个 NAS 工具；兼容 MCP（stdio/HTTP）。
- 渠道扩展：飞书、钉钉（企微/TG 已有底子）。
- 调度与主动汇报：把 `gateway_alerts.go` 的推送通道泛化为 cron 触发 skill 执行 + 结果推送。

### P4 — 记忆、审批、协作
- 记忆：SQLite + 本地向量（先 `sqlite-vec` 或简单 FTS，不引重依赖）。
- 审批卡片（IM 交互式消息）、密钥安全录入表单。
- 多 Bot 协作（放到最后：先让单个 Bot 真正有用）。

## 模块取舍（已在 feat/bot-direction 分支执行 ✅）

| 处置 | 模块 |
|---|---|
| 核心（保留） | setup users files system notification sudo terminal docker ai backup |
| 可选保留 | ssh（终端 SFTP 依赖）、sync、cloudbackup |
| 已移除 | photos video download samba retrogame vm android flatpak linuxlab translate |

已移除模块的替代路径：照片→应用商店装 Immich；视频→Jellyfin；下载/同步/Samba→应用商店容器化安装；VM/安卓→不做（云厂商与 umbrelOS 已更好）；翻译→大模型自带；Linux Lab/复古游戏→放弃。

## 不做清单（防战线回涨）

- 不自研 agent 编排内核（OpenClaw/Hermes 已卷完）；RDE 做"电脑+底座+渠道"。
- 不做容器内 systemd/DinD（`--privileged` 换不来等价价值）。
- 不做 flatpak/图形应用商店（被 Bot 工作台的桌面取代）。
- 界面文案不使用 "Grok Bot" 名称及衍生词（机制可借鉴，产品须自有命名）。
