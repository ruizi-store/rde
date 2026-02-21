// Draggable Action - Svelte Action
// 拖拽功能

export interface DraggableOptions {
  disabled?: boolean;
  onStart?: (e: { x: number; y: number; mouseX: number; mouseY: number }) => void;
  onMove?: (e: {
    x: number;
    y: number;
    dx: number;
    dy: number;
    mouseX: number;
    mouseY: number;
  }) => void;
  onEnd?: (e: { x: number; y: number; mouseX: number; mouseY: number }) => void;
}

export function draggable(node: HTMLElement, options: DraggableOptions = {}) {
  let startX = 0;
  let startY = 0;
  let initialX = 0;
  let initialY = 0;
  let isDragging = false;

  function handleMouseDown(e: MouseEvent) {
    if (options.disabled) return;
    if (e.button !== 0) return; // 只处理左键

    // 避免选中文本
    e.preventDefault();

    startX = e.clientX;
    startY = e.clientY;

    // 获取父窗口当前位置
    const parent = node.closest(".window") as HTMLElement;
    if (parent) {
      // 从 computed style 获取准确的位置
      const computedStyle = window.getComputedStyle(parent);
      initialX = parseFloat(computedStyle.left) || 0;
      initialY = parseFloat(computedStyle.top) || 0;
    }

    isDragging = true;
    options.onStart?.({ x: initialX, y: initialY, mouseX: e.clientX, mouseY: e.clientY });

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
  }

  function handleMouseMove(e: MouseEvent) {
    if (!isDragging) return;

    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    const newX = Math.max(0, initialX + dx);
    const newY = Math.max(0, initialY + dy);

    options.onMove?.({ x: newX, y: newY, dx, dy, mouseX: e.clientX, mouseY: e.clientY });
  }

  function handleMouseUp(e: MouseEvent) {
    if (!isDragging) return;

    isDragging = false;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;

    options.onEnd?.({
      x: Math.max(0, initialX + dx),
      y: Math.max(0, initialY + dy),
      mouseX: e.clientX,
      mouseY: e.clientY,
    });

    window.removeEventListener("mousemove", handleMouseMove);
    window.removeEventListener("mouseup", handleMouseUp);
  }

  node.addEventListener("mousedown", handleMouseDown);

  return {
    update(newOptions: DraggableOptions) {
      options = newOptions;
    },
    destroy() {
      node.removeEventListener("mousedown", handleMouseDown);
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    },
  };
}
