/**
 * Scrcpy Input Handler
 *
 * 捕获鼠标、触摸、滚轮、键盘事件，归一化坐标后通过回调发送控制消息。
 * 坐标归一化到 0~1 范围，后端负责乘以实际屏幕尺寸。
 */

// ==================== 控制消息类型 ====================

export interface TouchData {
  action: "down" | "up" | "move";
  x: number; // 0~1
  y: number; // 0~1
  pointerId: number;
  pressure: number;
  buttons: number;
}

export interface KeyData {
  keycode: number;
  action: "down" | "up";
}

export interface ScrollData {
  x: number; // 0~1
  y: number; // 0~1
  deltaY: number;
}

export type ControlMessage =
  | { type: "touch"; data: TouchData }
  | { type: "key"; data: KeyData }
  | { type: "scroll"; data: ScrollData }
  | { type: "back" }
  | { type: "home" }
  | { type: "recent" }
  | { type: "power" }
  | { type: "rotate" };

// ==================== Android KeyCode 映射 ====================

/** 常用 DOM key → Android AKEYCODE 映射 */
const KEY_MAP: Record<string, number> = {
  Backspace: 67,
  Enter: 66,
  Tab: 61,
  Escape: 111,
  Delete: 112,
  ArrowUp: 19,
  ArrowDown: 20,
  ArrowLeft: 21,
  ArrowRight: 22,
  Home: 3,
  End: 123,
  PageUp: 92,
  PageDown: 93,
  Space: 62,
  VolumeUp: 24,
  VolumeDown: 25,
};

/** a-z → AKEYCODE_A(29) ~ AKEYCODE_Z(54) */
function charToKeyCode(key: string): number | null {
  if (key.length === 1) {
    const code = key.toLowerCase().charCodeAt(0);
    if (code >= 97 && code <= 122) return code - 97 + 29; // a~z
    if (code >= 48 && code <= 57) return code - 48 + 7; // 0~9
  }
  return KEY_MAP[key] ?? null;
}

// ==================== InputHandler ====================

export interface InputHandlerCallbacks {
  onControl: (msg: ControlMessage) => void;
}

export class InputHandler {
  private element: HTMLElement | null = null;
  private callbacks: InputHandlerCallbacks | null = null;
  private boundHandlers: Array<{ target: EventTarget; event: string; handler: EventListener }> = [];
  private mouseDown = false;

  constructor() {}

  /**
   * 绑定到 HTML 元素，开始监听输入事件。
   */
  bind(element: HTMLElement, callbacks: InputHandlerCallbacks): void {
    this.unbind();
    this.element = element;
    this.callbacks = callbacks;

    // 鼠标事件
    this.on(element, "mousedown", this.handleMouseDown);
    this.on(element, "mousemove", this.handleMouseMove);
    this.on(element, "mouseup", this.handleMouseUp);
    this.on(element, "mouseleave", this.handleMouseUp);

    // 触摸事件
    this.on(element, "touchstart", this.handleTouchStart);
    this.on(element, "touchmove", this.handleTouchMove);
    this.on(element, "touchend", this.handleTouchEnd);
    this.on(element, "touchcancel", this.handleTouchEnd);

    // 滚轮
    this.on(element, "wheel", this.handleWheel);

    // 键盘（在 document 上监听，确保能捕获）
    this.on(document, "keydown", this.handleKeyDown);
    this.on(document, "keyup", this.handleKeyUp);

    // 禁止默认的触摸行为（如滚动、缩放）
    element.style.touchAction = "none";
  }

  /** 解绑所有事件监听 */
  unbind(): void {
    for (const { target, event, handler } of this.boundHandlers) {
      target.removeEventListener(event, handler);
    }
    this.boundHandlers = [];
    if (this.element) {
      this.element.style.touchAction = "";
    }
    this.element = null;
    this.callbacks = null;
    this.mouseDown = false;
  }

  /** 发送导航按钮 */
  sendBack(): void {
    this.emit({ type: "back" });
  }
  sendHome(): void {
    this.emit({ type: "home" });
  }
  sendRecent(): void {
    this.emit({ type: "recent" });
  }
  sendPower(): void {
    this.emit({ type: "power" });
  }
  sendRotate(): void {
    this.emit({ type: "rotate" });
  }

  /** 销毁 */
  destroy(): void {
    this.unbind();
  }

  // ============ 私有方法 ============

  private emit(msg: ControlMessage): void {
    this.callbacks?.onControl(msg);
  }

  private on(target: EventTarget, event: string, handler: (e: Event) => void): void {
    const bound = handler.bind(this) as EventListener;
    target.addEventListener(event, bound, { passive: false });
    this.boundHandlers.push({ target, event, handler: bound });
  }

  /**
   * 将客户端坐标归一化到 0~1。
   */
  private normalize(clientX: number, clientY: number): { x: number; y: number } {
    if (!this.element) return { x: 0, y: 0 };
    const rect = this.element.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(1, (clientX - rect.left) / rect.width)),
      y: Math.max(0, Math.min(1, (clientY - rect.top) / rect.height)),
    };
  }

  // -- 鼠标 --

  private handleMouseDown(e: Event): void {
    const me = e as MouseEvent;
    me.preventDefault();
    this.mouseDown = true;
    const { x, y } = this.normalize(me.clientX, me.clientY);
    this.emit({ type: "touch", data: { action: "down", x, y, pointerId: 0, pressure: 1.0, buttons: 1 } });
  }

  private handleMouseMove(e: Event): void {
    if (!this.mouseDown) return;
    const me = e as MouseEvent;
    me.preventDefault();
    const { x, y } = this.normalize(me.clientX, me.clientY);
    this.emit({ type: "touch", data: { action: "move", x, y, pointerId: 0, pressure: 1.0, buttons: 1 } });
  }

  private handleMouseUp(e: Event): void {
    if (!this.mouseDown) return;
    const me = e as MouseEvent;
    me.preventDefault();
    this.mouseDown = false;
    const { x, y } = this.normalize(me.clientX, me.clientY);
    this.emit({ type: "touch", data: { action: "up", x, y, pointerId: 0, pressure: 0, buttons: 0 } });
  }

  // -- 触摸 --

  private handleTouchStart(e: Event): void {
    const te = e as TouchEvent;
    te.preventDefault();
    for (let i = 0; i < te.changedTouches.length; i++) {
      const t = te.changedTouches[i];
      const { x, y } = this.normalize(t.clientX, t.clientY);
      this.emit({
        type: "touch",
        data: { action: "down", x, y, pointerId: t.identifier, pressure: t.force || 1.0, buttons: 1 },
      });
    }
  }

  private handleTouchMove(e: Event): void {
    const te = e as TouchEvent;
    te.preventDefault();
    for (let i = 0; i < te.changedTouches.length; i++) {
      const t = te.changedTouches[i];
      const { x, y } = this.normalize(t.clientX, t.clientY);
      this.emit({
        type: "touch",
        data: { action: "move", x, y, pointerId: t.identifier, pressure: t.force || 1.0, buttons: 1 },
      });
    }
  }

  private handleTouchEnd(e: Event): void {
    const te = e as TouchEvent;
    te.preventDefault();
    for (let i = 0; i < te.changedTouches.length; i++) {
      const t = te.changedTouches[i];
      const { x, y } = this.normalize(t.clientX, t.clientY);
      this.emit({
        type: "touch",
        data: { action: "up", x, y, pointerId: t.identifier, pressure: 0, buttons: 0 },
      });
    }
  }

  // -- 滚轮 --

  private handleWheel(e: Event): void {
    const we = e as WheelEvent;
    we.preventDefault();
    const { x, y } = this.normalize(we.clientX, we.clientY);
    // deltaY 归一化：正值向下滚动
    const deltaY = Math.sign(we.deltaY) * -1; // scrcpy 协议中正值=向上
    this.emit({ type: "scroll", data: { x, y, deltaY } });
  }

  // -- 键盘 --

  private handleKeyDown(e: Event): void {
    const ke = e as KeyboardEvent;
    const keycode = charToKeyCode(ke.key);
    if (keycode !== null) {
      ke.preventDefault();
      this.emit({ type: "key", data: { keycode, action: "down" } });
    }
  }

  private handleKeyUp(e: Event): void {
    const ke = e as KeyboardEvent;
    const keycode = charToKeyCode(ke.key);
    if (keycode !== null) {
      ke.preventDefault();
      this.emit({ type: "key", data: { keycode, action: "up" } });
    }
  }
}
