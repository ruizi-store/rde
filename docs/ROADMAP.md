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
- ~~后续：setup 向导在容器模式跳过 netplan/挂载/PAM/端口选择~~ ✅ 2026-11（0.6.0）：免登录模式 `RDE_AUTH_MODE=none`（默认）
  直接跳过整个向导与账号注册，起容器即用；`local` 模式保留向导+登录。DEB 交付已移除，Docker 镜像为唯一交付形态。
- 后续：CI 加 buildx 多架构构建并推镜像仓库。

### P2 — Bot 工作台（最小可用 ✅ 2026-11）
- 新模块 `modules/bot`：Bot 的 CRUD（名字/职责/模型/成员可见性），接管互斥锁。✅
- Bot 沙箱容器：复用 `docker` 模块 client；镜像 `rde/bot-desktop`（Debian trixie + Chromium + KasmVNC，
  KasmVNC deb 来自 `thirdparty/kasmvnc`，`make bot-desktop` 构建）。✅
- 三级可见性：状态（图标点亮）/ 预览（iframe 围观 KasmVNC 串流）/ 接管（互斥 + 30 分钟过期）。✅
- 工具调用走沙箱 exec（`ExecWithTimeout`：超时 300s 上限 / 输出 64KB 截断；控制面不执行用户脚本）。✅
- 网络拓扑：`rde-bot-net` bridge 网络，控制面容器与沙箱同网段（compose 已配置）。✅

### P3 — 技能与渠道（✅ 2026-11，MCP/飞书/钉钉/cron 留 P4）
- 技能文件化：`/var/lib/rde/skills/<name>/SKILL.md`（对齐 agent skills 生态格式：frontmatter name/description + 正文指令 + 可选 `scripts/run.sh|run.py`）；
  首次启动播种默认技能（web-research / daily-report），REST 热加载（`GET/POST /bots/skills`）。✅
- 渐进式披露：网关对话 system prompt 只注入技能名+简介索引，LLM 按需 `get_skill` 读全文，`execute_script` 在该 Bot 沙箱执行，
  `run_skill` 直接跑预置脚本（参数经 base64 注入 `SKILL_ARGS`，防命令行注入）。✅
- Bot 与 AI 网关打通：`Bot.Channels`（JSON）按 platform + chat_id/user_id 把 IM 消息路由到对应 Bot；
  Bot 自带 provider/model/职责 system prompt（`ai/bot_bridge.go` 接口 + bot 模块注入，依赖方向 bot→ai 不反向）。✅
- 网关回调安全：webhook/telegram/wecom 三个外部端点放开 JWT（各自带 API Key / secret_token / 消息签名鉴权；
  Telegram 新增 `WebhookSecret`，未配置时拒绝回调）。✅
- 前端：Bot 详情新增「渠道」绑定编辑与「技能」清单标签页。✅
- 遗留：`skills.go` 的 22 个硬编码 NAS 工具继续并存（网关默认助手路径仍用）；alerts 泛化为 cron 技能调度。
  （MCP 兼容、飞书/钉钉渠道已完成，见 P4 ✅ 2026-11）

### P4 — 记忆、审批、协作（进行中）
- 记忆 ✅ 2026-11：`bots_memories` 表 + `save_memory`/`search_memory` 工具（LLM 自主存取，同 key 覆盖，
  关键词 LIKE 检索），REST 审计（查看/搜索/删除）；system prompt 提示先查后存。
- 定时主动汇报 ✅ 2026-11：`bots_schedules` 表（robfig/cron 5 字段 + `@every` 描述符），
  触发时以 Bot 身份跑带工具对话（技能/沙箱/记忆可用），结果推送到 Bot 绑定渠道；
  支持 RunNow 手动执行、暂停、上次结果/错误落库；前端「日程」「记忆」标签页。
- 审批 ✅ 2026-11：`bots_approvals` 表 + `request_approval` 工具（高风险操作先请示）。审批请求推送到
  Bot 绑定渠道（IM 回复 `/approve|/deny <id>`）并出现在 Web「审批」标签页；工具阻塞等待
  （waiter channel + 2s 兜底轮询，默认 120s 上限 300s），批准后 Bot 继续执行，拒绝/超时即放弃；
  已处理审批不可重复决定；Bot 工具调用轮数上限放宽到 12（审批链路更长）。
- 审批卡片升级为 IM 交互式消息 ✅ 2026-11（0.7.0）：Telegram inline keyboard、企微 button_list
  模板卡片、飞书 interactive 卡片按钮（回调直接决定，DecideApprovalByID）；钉钉 actionCard 附 Web
  审批页深链；文本命令 /approve|/deny 全平台兜底。
- 渠道扩展 ✅ 2026-11（0.7.0）：飞书（自建应用机器人：tenant_access_token 发送 + 事件订阅回调）、
  钉钉（Stream 模式 WebSocket 长连接，无需公网回调）。
- MCP 兼容 ✅ 2026-11（0.7.0）：Streamable HTTP + stdio 两种传输，REST 配置管理，
  工具以 mcp_<server>_<tool> 桥接进 Bot 工具列表（可信基础设施，不经沙箱）。
- none→local 升级入口 ✅ 2026-11（0.7.0）：设置页「登录方式」卡片，创建管理员并写
  rde.conf [auth] mode=local（重启生效）；认证模式支持 conf（优先级 env > conf > 默认 none）。
- 待办：密钥安全录入表单、多 Bot 协作（放最后：先让单个 Bot 真正有用）。
- 清理 `gateway_alerts.go` 泛化为通用调度推送（与 bots_schedules 并存一段时间）。

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
