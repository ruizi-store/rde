// 套件系统类型定义
// 用于 .rzos 套件的前端集成

/**
 * 套件窗口配置
 */
export interface PackageWindowConfig {
  title: string;
  icon: string;
  width: number;
  height: number;
  minWidth?: number;
  minHeight?: number;
  maxWidth?: number;
  maxHeight?: number;
  resizable?: boolean;
  singleton?: boolean;
  position?: "center" | "cascade";
}

/**
 * 套件前端配置
 */
export interface PackageFrontendConfig {
  entry: string; // iframe 入口 URL，如 "/packages/docker-appstore/index.html"
  window: PackageWindowConfig;
}

/**
 * 已安装套件信息 (从后端获取)
 */
export interface InstalledPackage {
  id: string;
  name: string;
  version: string;
  description: string;
  icon: string;
  status: "installed" | "running" | "stopped" | "error";
  frontend?: PackageFrontendConfig;
  hasBackend: boolean;
  installedAt: string;
}

/**
 * iframe 通信消息类型
 */
export type IframeMessageType =
  | "API_REQUEST"
  | "API_RESPONSE"
  | "WINDOW_CONTROL"
  | "WINDOW_RESIZE"
  | "WINDOW_EVENT";

/**
 * API 请求消息
 */
export interface ApiRequestMessage {
  type: "API_REQUEST";
  id: string;
  payload: {
    service: string;
    method: string;
    params?: Record<string, unknown>;
  };
}

/**
 * API 响应消息
 */
export interface ApiResponseMessage {
  type: "API_RESPONSE";
  id: string;
  success: boolean;
  result?: unknown;
  error?: string;
}

/**
 * 窗口控制消息
 */
export interface WindowControlMessage {
  type: "WINDOW_CONTROL";
  payload: {
    action: "setTitle" | "close" | "minimize" | "maximize" | "resize" | "showNotification";
    data?: unknown;
  };
}

/**
 * 窗口大小变化消息
 */
export interface WindowResizeMessage {
  type: "WINDOW_RESIZE";
  payload: {
    width: number;
    height: number;
  };
}

/**
 * 窗口事件消息
 */
export interface WindowEventMessage {
  type: "WINDOW_EVENT";
  payload: {
    event: "focus" | "blur" | "close";
  };
}

/**
 * 所有 iframe 消息类型联合
 */
export type IframeMessage =
  | ApiRequestMessage
  | ApiResponseMessage
  | WindowControlMessage
  | WindowResizeMessage
  | WindowEventMessage;

/**
 * 套件权限
 */
export type PackagePermission =
  | "file:read"
  | "file:write"
  | "user:read"
  | "user:write"
  | "system:info"
  | "system:control"
  | "network:local"
  | "network:internet"
  | "docker:read"
  | "docker:write"
  | "notification:send"
  | string; // 允许自定义权限
