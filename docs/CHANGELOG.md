rde (0.6.0) unstable; urgency=medium

  * feat(auth): 免登录单用户模式 RDE_AUTH_MODE=none（默认）——起容器即进桌面，
    无需安装向导/注册账号；none 模式下 setup 幂等自动完成、JWT 中间件替换为
    固定管理员身份注入（审计字段与 RequireAdmin 照常工作）；
    设 RDE_AUTH_MODE=local 恢复向导+登录；/setup/status 新增 auth_mode 字段
  * feat(frontend): 根路由/登录页按 auth_mode 自动路由——none 直接进桌面，
    local 走原向导/登录流程
  * feat(docker): compose 显式 RDE_AUTH_MODE 配置与免登录部署说明
  * chore(delivery): 移除 DEB 交付（debian/ 删除），Docker 镜像为唯一交付形态；
    版本历史迁移至 docs/CHANGELOG.md；Makefile 去掉 deb/deploy 目标
  * main.go：none 模式视为已初始化（端口决策读配置）

 -- RDE Maintainers <maintainers@rde.local>  Tue, 24 Nov 2026 21:00:00 +0800

rde (0.5.0) unstable; urgency=medium

  * feat(bot): 高风险操作人工审批——bots_approvals 表 + request_approval 工具，
    LLM 执行敏感操作前先请示；审批请求推送 Bot 绑定渠道（IM 回复 /approve|/deny <id>）
    并出现在 Web 审批标签页；工具阻塞等待（waiter + 2s 兜底轮询，默认 120s 上限 300s），
    拒绝/超时即放弃执行；已处理审批不可重复决定
  * feat(ai): ApprovalManager 桥接接口与网关 /approve /deny 命令路由（只对绑定 Bot 的会话生效）；
    system prompt 补审批行为指引；Bot 工具调用轮数上限 5 → 12（审批链路更长）
  * feat(frontend): Bot 详情新增「审批」标签页（待审/全部过滤、状态徽章、批准/拒绝）
  * feat(api): GET /bots/:id/approvals、POST /bots/:id/approvals/:aid/decide

 -- RDE Maintainers <maintainers@rde.local>  Tue, 24 Nov 2026 20:00:00 +0800

rde (0.4.0) unstable; urgency=medium

  * feat(bot): Bot 长期记忆——bots_memories 表 + save_memory/search_memory 工具
    （LLM 对话中自主存取，REST 审计查看/搜索/删除）
  * feat(bot): 定时主动汇报——bots_schedules 表（robfig/cron，5 字段 + @every 描述符），
    到点以 Bot 身份执行带工具任务（技能/沙箱/记忆可用）并把结果推送到绑定 IM 渠道；
    支持立即执行、暂停、上次结果落库
  * feat(frontend): Bot 详情新增「日程」「记忆」标签页
  * feat(ai): Module.RunBotTask / SendToChannel 导出（调度入口）；记忆接口 MemoryProvider

 -- RDE Maintainers <maintainers@rde.local>  Tue, 24 Nov 2026 19:00:00 +0800

rde (0.3.0) unstable; urgency=medium

  * feat(bot): 技能文件化——SKILL.md（frontmatter+正文+预置脚本），首次播种默认技能，REST 热加载
  * feat(bot): IM 渠道绑定（Bot.Channels），网关消息按 platform+chat_id/user_id 路由到对应 Bot 应答
  * feat(ai): Bot 沙箱桥接接口（SandboxExecutor/SkillProvider/BotResolver），工具 execute_script/run_skill
    在 Bot 沙箱内执行；渐进式披露注入技能索引（list_skills/get_skill）
  * feat(ai): 网关外部回调端点（webhook/telegram/wecom）放开 JWT，各自带 API Key/secret_token/签名鉴权；
    Telegram 新增 WebhookSecret 防伪造回调
  * feat(frontend): Bot 详情新增渠道绑定编辑与技能清单标签页
  * chore(docker): 镜像版本 0.3.0-p3，文档补充技能目录与渠道路由说明

 -- RDE Maintainers <maintainers@rde.local>  Tue, 24 Nov 2026 18:00:00 +0800

rde (0.2.4) unstable; urgency=medium

  * feat(linuxlab): 构建日志环形缓冲，刷新后可重连恢复历史与实时输出
  * feat(desktop): 关机/重启全屏遮罩与健康探测，避免前端「假活」
  * fix(ui): Linux Lab / Tabs 日志区域滚动条可正常拖动

 -- liduanjun <duanjunzi@gmail.com>  Sun, 19 Jul 2026 19:57:06 +0800

rde (0.2.3) unstable; urgency=medium

  * feat(docker): 已安装应用展示端口与打开入口，移除无意义桌面图标
  * fix(flatpak): 国内默认源改用中科大，修复交大镜像 SSL [60]
  * fix(translate): 离线模型灌入后修正 volume 属主，避免容器崩溃重启
  * fix(translate): 离线预置 en/zh 模型并加固 ensure / 离线优先启动
  * fix(files): 分片上传改用数据盘临时目录；修复多文件窗口剪切与拖拽
  * fix(android): 投屏 WebSocket 附带 JWT，修复鉴权导致黑屏
  * fix(web): 用安装实例 ID 对账，消除同 IP 换机残留前端缓存

 -- liduanjun <duanjunzi@gmail.com>  Sun, 19 Jul 2026 19:09:34 +0800

rde (0.2.2) unstable; urgency=medium

  * fix(android): 离线包优先准备镜像（本地 Docker → offline tar → pull）
  * feat(offline): HasImage 检测；manifest 增加 libretranslate

 -- liduanjun <duanjunzi@gmail.com>  Sun, 19 Jul 2026 10:41:59 +0800

rde (0.2.1) unstable; urgency=medium

  * fix(deb): thirdparty/offline 安装到 /usr/share/rde/offline
  * feat: 离线资源框架（Docker 镜像优先本地加载）
  * feat: 内置 EmulatorJS / KasmVNC / Iconify，减少运行时联网
  * feat: AI / VM / Android / Linux Lab 等能力并入核心模块

 -- liduanjun <duanjunzi@gmail.com>  Sun, 19 Jul 2026 09:10:00 +0800

rde (0.2.0) unstable; urgency=medium

  * fix: flatpak 应用若干bug

 -- liduanjun <duanjunzi@gmail.com>  Mon, 02 Mar 2026 10:40:20 +0800

rde (0.1.0-1) unstable; urgency=medium

  * Initial open source release v0.1.0

 -- liduanjun <duanjunzi@gmail.com>  Sat, 21 Feb 2026 22:00:00 +0800
