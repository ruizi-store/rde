// UI 状态 Store - 控制全局 UI 面板状态
// 用于组件间通信，如泡泡点击后打开通知面板

class UIStateStore {
  // 通知面板是否应该打开
  private _notificationPanelRequested = $state(false);

  // 请求打开通知面板
  requestOpenNotificationPanel(): void {
    this._notificationPanelRequested = true;
  }

  // 消费请求（TaskBar 调用）
  consumeNotificationPanelRequest(): boolean {
    if (this._notificationPanelRequested) {
      this._notificationPanelRequested = false;
      return true;
    }
    return false;
  }

  // 检查是否有请求
  get hasNotificationPanelRequest(): boolean {
    return this._notificationPanelRequested;
  }
}

export const uiState = new UIStateStore();
