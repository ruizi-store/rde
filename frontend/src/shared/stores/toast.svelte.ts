// Toast 通知状态管理
// 全局 Toast 消息队列

import { generateUUID } from "$shared/utils/uuid";

export interface ToastMessage {
  id: string;
  type: "info" | "success" | "warning" | "error";
  title: string;
  message?: string;
  duration?: number; // 毫秒
  closable?: boolean;
}

class ToastStore {
  // Toast 消息列表
  messages = $state<ToastMessage[]>([]);

  // 默认持续时间（毫秒）
  defaultDuration = 5000;

  // 添加 toast
  add(toast: Omit<ToastMessage, "id">): string {
    const id = generateUUID();
    const duration = toast.duration ?? this.defaultDuration;

    this.messages = [...this.messages, { ...toast, id, closable: toast.closable ?? true }];

    // 自动关闭
    if (duration > 0) {
      setTimeout(() => this.remove(id), duration);
    }

    return id;
  }

  // 移除 toast
  remove(id: string): void {
    this.messages = this.messages.filter((t) => t.id !== id);
  }

  // 清空所有
  clear(): void {
    this.messages = [];
  }

  // 便捷方法
  info(title: string, message?: string, duration?: number): string {
    return this.add({ type: "info", title, message, duration });
  }

  success(title: string, message?: string, duration?: number): string {
    return this.add({ type: "success", title, message, duration });
  }

  warning(title: string, message?: string, duration?: number): string {
    return this.add({ type: "warning", title, message, duration });
  }

  error(title: string, message?: string, duration?: number): string {
    return this.add({ type: "error", title, message, duration });
  }
}

export const toast = new ToastStore();
