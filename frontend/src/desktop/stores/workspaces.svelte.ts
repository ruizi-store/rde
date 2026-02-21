// 工作区状态管理
// 实现虚拟桌面/多工作区功能

import { generateUUID } from "$shared/utils/uuid";

export interface Workspace {
  id: string;
  name: string;
  windowIds: string[]; // 该工作区中的窗口 ID
}

class WorkspaceStore {
  // 工作区列表
  private _workspaces = $state<Workspace[]>([{ id: "main", name: "工作区 1", windowIds: [] }]);

  // 当前活动工作区 ID
  private _activeId = $state("main");

  // 最大工作区数量
  readonly maxWorkspaces = 6;

  // 获取所有工作区
  get all(): Workspace[] {
    return this._workspaces;
  }

  // 获取当前活动工作区
  get active(): Workspace {
    return this._workspaces.find((w) => w.id === this._activeId) || this._workspaces[0];
  }

  // 获取活动工作区 ID
  get activeId(): string {
    return this._activeId;
  }

  // 获取工作区数量
  get count(): number {
    return this._workspaces.length;
  }

  // 创建新工作区
  create(name?: string): string | null {
    if (this._workspaces.length >= this.maxWorkspaces) {
      return null;
    }

    const id = generateUUID();
    const newWorkspace: Workspace = {
      id,
      name: name || `工作区 ${this._workspaces.length + 1}`,
      windowIds: [],
    };

    this._workspaces = [...this._workspaces, newWorkspace];
    return id;
  }

  // 删除工作区
  delete(id: string): boolean {
    if (this._workspaces.length <= 1) {
      return false; // 至少保留一个工作区
    }

    const index = this._workspaces.findIndex((w) => w.id === id);
    if (index === -1) return false;

    // 如果删除的是当前活动工作区，切换到相邻工作区
    if (this._activeId === id) {
      const newIndex = index > 0 ? index - 1 : 1;
      this._activeId = this._workspaces[newIndex].id;
    }

    this._workspaces = this._workspaces.filter((w) => w.id !== id);
    return true;
  }

  // 重命名工作区
  rename(id: string, name: string): void {
    this._workspaces = this._workspaces.map((w) => (w.id === id ? { ...w, name } : w));
  }

  // 切换到工作区
  switchTo(id: string): void {
    if (this._workspaces.some((w) => w.id === id)) {
      this._activeId = id;
    }
  }

  // 切换到上一个工作区
  prev(): void {
    const index = this._workspaces.findIndex((w) => w.id === this._activeId);
    if (index > 0) {
      this._activeId = this._workspaces[index - 1].id;
    }
  }

  // 切换到下一个工作区
  next(): void {
    const index = this._workspaces.findIndex((w) => w.id === this._activeId);
    if (index < this._workspaces.length - 1) {
      this._activeId = this._workspaces[index + 1].id;
    }
  }

  // 将窗口添加到当前工作区
  addWindow(windowId: string): void {
    const workspace = this.active;
    if (!workspace.windowIds.includes(windowId)) {
      workspace.windowIds = [...workspace.windowIds, windowId];
      // 触发响应式更新
      this._workspaces = [...this._workspaces];
    }
  }

  // 从工作区移除窗口
  removeWindow(windowId: string): void {
    this._workspaces = this._workspaces.map((w) => ({
      ...w,
      windowIds: w.windowIds.filter((id) => id !== windowId),
    }));
  }

  // 将窗口移动到指定工作区
  moveWindowTo(windowId: string, workspaceId: string): void {
    this._workspaces = this._workspaces.map((w) => {
      if (w.id === workspaceId) {
        // 添加到目标工作区
        if (!w.windowIds.includes(windowId)) {
          return { ...w, windowIds: [...w.windowIds, windowId] };
        }
      } else {
        // 从其他工作区移除
        return { ...w, windowIds: w.windowIds.filter((id) => id !== windowId) };
      }
      return w;
    });
  }

  // 检查窗口是否在当前工作区
  isWindowInActive(windowId: string): boolean {
    return this.active.windowIds.includes(windowId);
  }

  // 获取窗口所在的工作区
  getWindowWorkspace(windowId: string): Workspace | undefined {
    return this._workspaces.find((w) => w.windowIds.includes(windowId));
  }
}

export const workspaces = new WorkspaceStore();
