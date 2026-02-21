/**
 * Iframe Bridge - 主框架与套件 iframe 之间的通信桥梁
 *
 * 处理来自套件的 postMessage 消息，并提供相应的功能代理：
 * - 导航
 * - 通知
 * - API 请求代理
 * - 状态同步
 * - 事件广播
 */

import { goto } from "$app/navigation";
import { toast } from "$shared/stores/toast.svelte";
import { userStore } from "$shared/stores/user.svelte";
import { theme } from "$shared/stores/theme.svelte";
import { windowManager } from "$desktop/stores/windows.svelte";

// 消息类型
type MessageType =
  | "init"
  | "ready"
  | "navigate"
  | "notify"
  | "request"
  | "response"
  | "event"
  | "state"
  | "error"
  | "resize"
  | "focus"
  | "blur";

// 基础消息结构
interface Message<T = unknown> {
  type: MessageType;
  id: string;
  payload: T;
  timestamp: number;
  source: string; // 来源套件 ID
}

// 初始化消息
interface InitPayload {
  packageId: string;
  packageName?: string;
  version?: string;
  theme: "light" | "dark" | "system";
  locale: string;
  token?: string;
}

// 导航消息
interface NavigatePayload {
  path: string;
  params?: Record<string, string>;
  replace?: boolean;
}

// 通知消息
interface NotifyPayload {
  title: string;
  message: string;
  type: "info" | "success" | "warning" | "error";
  duration?: number;
  action?: {
    label: string;
    callback: string;
  };
}

// API 请求消息
interface RequestPayload {
  method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  url: string;
  headers?: Record<string, string>;
  body?: unknown;
}

// API 响应消息
interface ResponsePayload {
  requestId: string;
  status: number;
  headers: Record<string, string>;
  data: unknown;
  error?: string;
}

// 调整大小消息
interface ResizePayload {
  width: number;
  height: number;
  fullscreen?: boolean;
}

// 已注册的 iframe
interface RegisteredIframe {
  packageId: string;
  iframe: HTMLIFrameElement;
  windowId: string;
  ready: boolean;
}

class IframeBridge {
  private iframes = new Map<string, RegisteredIframe>();
  private messageListener: ((event: MessageEvent) => void) | null = null;
  private eventHandlers = new Map<string, Set<(data: unknown) => void>>();

  constructor() {
    this.setupMessageListener();
  }

  /**
   * 注册 iframe
   */
  register(packageId: string, iframe: HTMLIFrameElement, windowId: string): void {
    this.iframes.set(packageId, {
      packageId,
      iframe,
      windowId,
      ready: false,
    });

    // 发送初始化消息
    this.sendInit(packageId);
  }

  /**
   * 注销 iframe
   */
  unregister(packageId: string): void {
    this.iframes.delete(packageId);
  }

  /**
   * 获取已注册的 iframe
   */
  getIframe(packageId: string): HTMLIFrameElement | null {
    return this.iframes.get(packageId)?.iframe ?? null;
  }

  /**
   * 发送消息到指定套件
   */
  send<T>(packageId: string, type: MessageType, payload: T): void {
    const registered = this.iframes.get(packageId);
    if (!registered || !registered.iframe.contentWindow) {
      console.warn(`[IframeBridge] Package not found: ${packageId}`);
      return;
    }

    const message: Message<T> = {
      type,
      id: this.generateId(),
      payload,
      timestamp: Date.now(),
      source: "rde-host",
    };

    registered.iframe.contentWindow.postMessage(message, "*");
  }

  /**
   * 广播消息到所有套件
   */
  broadcast<T>(type: MessageType, payload: T): void {
    for (const [packageId] of this.iframes) {
      this.send(packageId, type, payload);
    }
  }

  /**
   * 订阅全局事件
   */
  on(eventName: string, handler: (data: unknown) => void): () => void {
    if (!this.eventHandlers.has(eventName)) {
      this.eventHandlers.set(eventName, new Set());
    }
    this.eventHandlers.get(eventName)!.add(handler);
    return () => {
      this.eventHandlers.get(eventName)?.delete(handler);
    };
  }

  /**
   * 通知主题变化
   */
  notifyThemeChange(newTheme: "light" | "dark" | "system"): void {
    this.broadcast("state", { key: "theme", value: newTheme });
  }

  /**
   * 通知语言变化
   */
  notifyLocaleChange(newLocale: string): void {
    this.broadcast("state", { key: "locale", value: newLocale });
  }

  /**
   * 通知 token 变化
   */
  notifyTokenChange(newToken: string): void {
    this.broadcast("state", { key: "token", value: newToken });
  }

  /**
   * 通知窗口聚焦
   */
  notifyFocus(packageId: string): void {
    this.send(packageId, "focus", {});
  }

  /**
   * 通知窗口失焦
   */
  notifyBlur(packageId: string): void {
    this.send(packageId, "blur", {});
  }

  /**
   * 销毁
   */
  destroy(): void {
    if (this.messageListener) {
      window.removeEventListener("message", this.messageListener);
      this.messageListener = null;
    }
    this.iframes.clear();
    this.eventHandlers.clear();
  }

  // ========== 私有方法 ==========

  /**
   * 设置消息监听
   */
  private setupMessageListener(): void {
    this.messageListener = (event: MessageEvent) => {
      const message = event.data as Message;
      if (!message || !message.type || !message.id) {
        return;
      }

      // 验证消息来源
      const registered = this.findByOrigin(event.source as Window);
      if (!registered) {
        console.warn("[IframeBridge] Message from unknown source");
        return;
      }

      console.log(`[IframeBridge] Received from ${registered.packageId}:`, message.type);
      this.handleMessage(registered, message);
    };

    // 只在浏览器环境中添加事件监听器
    if (typeof window !== "undefined") {
      window.addEventListener("message", this.messageListener);
    }
  }

  /**
   * 根据 window 对象查找注册的 iframe
   */
  private findByOrigin(source: Window): RegisteredIframe | null {
    for (const registered of this.iframes.values()) {
      if (registered.iframe.contentWindow === source) {
        return registered;
      }
    }
    return null;
  }

  /**
   * 处理消息
   */
  private handleMessage(registered: RegisteredIframe, message: Message): void {
    switch (message.type) {
      case "init":
        this.handleInit(registered, message);
        break;

      case "ready":
        registered.ready = true;
        console.log(`[IframeBridge] Package ${registered.packageId} is ready`);
        break;

      case "navigate":
        this.handleNavigate(message as Message<NavigatePayload>);
        break;

      case "notify":
        this.handleNotify(message as Message<NotifyPayload>);
        break;

      case "request":
        this.handleRequest(registered.packageId, message as Message<RequestPayload>);
        break;

      case "event":
        this.handleEvent(message as Message<{ name: string; data: unknown }>);
        break;

      case "resize":
        this.handleResize(registered, message as Message<ResizePayload>);
        break;

      default:
        console.warn(`[IframeBridge] Unknown message type: ${message.type}`);
    }
  }

  /**
   * 处理初始化
   */
  private handleInit(registered: RegisteredIframe, message: Message): void {
    // 套件请求初始化，发送配置
    this.sendInit(registered.packageId);
  }

  /**
   * 发送初始化配置
   */
  private sendInit(packageId: string): void {
    const payload: InitPayload = {
      packageId,
      theme: theme.isDark ? "dark" : "light",
      locale: "zh-CN",
      token: userStore.token ?? undefined,
    };

    this.send(packageId, "init", payload);
  }

  /**
   * 处理导航请求
   */
  private handleNavigate(message: Message<NavigatePayload>): void {
    const { path, params, replace } = message.payload;

    let url = path;
    if (params) {
      const searchParams = new URLSearchParams(params);
      url = `${path}?${searchParams.toString()}`;
    }

    goto(url, { replaceState: replace });
  }

  /**
   * 处理通知请求
   */
  private handleNotify(message: Message<NotifyPayload>): void {
    const { title, message: msg, type, duration } = message.payload;

    switch (type) {
      case "success":
        toast.success(title, msg, duration);
        break;
      case "error":
        toast.error(title, msg, duration);
        break;
      case "warning":
        toast.warning(title, msg, duration);
        break;
      case "info":
      default:
        toast.info(title, msg, duration);
        break;
    }
  }

  /**
   * 处理 API 请求
   */
  private async handleRequest(packageId: string, message: Message<RequestPayload>): Promise<void> {
    const { method, url, headers, body } = message.payload;

    try {
      const fetchOptions: RequestInit = {
        method,
        headers: {
          "Content-Type": "application/json",
          ...headers,
          // 添加认证 token
          ...(userStore.token ? { Authorization: `Bearer ${userStore.token}` } : {}),
        },
      };

      if (body && method !== "GET") {
        fetchOptions.body = JSON.stringify(body);
      }

      const response = await fetch(url, fetchOptions);
      const data = await response.json().catch(() => null);

      const responsePayload: ResponsePayload = {
        requestId: message.id,
        status: response.status,
        headers: Object.fromEntries(response.headers.entries()),
        data: data,
      };

      if (!response.ok) {
        responsePayload.error = data?.message || `HTTP ${response.status}`;
      }

      this.send(packageId, "response", responsePayload);
    } catch (error) {
      const responsePayload: ResponsePayload = {
        requestId: message.id,
        status: 0,
        headers: {},
        data: null,
        error: error instanceof Error ? error.message : "Unknown error",
      };

      this.send(packageId, "response", responsePayload);
    }
  }

  /**
   * 处理事件
   */
  private handleEvent(message: Message<{ name: string; data: unknown }>): void {
    const { name, data } = message.payload;

    // 触发本地事件处理器
    const handlers = this.eventHandlers.get(name);
    if (handlers) {
      for (const handler of handlers) {
        try {
          handler(data);
        } catch (err) {
          console.error("[IframeBridge] Event handler error:", err);
        }
      }
    }

    // 广播到其他套件
    this.broadcast("event", message.payload);
  }

  /**
   * 处理窗口调整大小
   */
  private handleResize(registered: RegisteredIframe, message: Message<ResizePayload>): void {
    const { width, height, fullscreen } = message.payload;

    if (fullscreen) {
      windowManager.toggleMaximize(registered.windowId);
    } else if (width > 0 && height > 0) {
      windowManager.resize(registered.windowId, width, height);
    }
  }

  /**
   * 生成唯一 ID
   */
  private generateId(): string {
    return `host-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
  }
}

// 导出延迟初始化的单例，避免 SSR 时 window 未定义
let _iframeBridge: IframeBridge | null = null;

export const iframeBridge = {
  get instance(): IframeBridge {
    if (!_iframeBridge && typeof window !== "undefined") {
      _iframeBridge = new IframeBridge();
    }
    return _iframeBridge!;
  },

  register(packageId: string, iframe: HTMLIFrameElement, windowId: string): void {
    if (typeof window !== "undefined") {
      this.instance.register(packageId, iframe, windowId);
    }
  },

  unregister(packageId: string): void {
    if (typeof window !== "undefined") {
      this.instance.unregister(packageId);
    }
  },
};
