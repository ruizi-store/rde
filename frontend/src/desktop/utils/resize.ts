// Resizable Action - Svelte Action
// 调整大小功能

export type ResizeDirection = "n" | "s" | "e" | "w" | "ne" | "nw" | "se" | "sw";

export interface ResizableOptions {
  direction: ResizeDirection;
  minWidth?: number;
  minHeight?: number;
  onStart?: () => void;
  onResize?: (e: { width: number; height: number; x?: number; y?: number }) => void;
  onEnd?: () => void;
}

export function resizable(node: HTMLElement, options: ResizableOptions) {
  let startX = 0;
  let startY = 0;
  let initialWidth = 0;
  let initialHeight = 0;
  let initialX = 0;
  let initialY = 0;
  let isResizing = false;

  const minWidth = options.minWidth ?? 200;
  const minHeight = options.minHeight ?? 150;

  function handleMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;

    e.preventDefault();
    e.stopPropagation();

    const parent = node.closest(".window") as HTMLElement;
    if (!parent) return;

    startX = e.clientX;
    startY = e.clientY;
    initialWidth = parent.offsetWidth;
    initialHeight = parent.offsetHeight;
    initialX = parent.offsetLeft;
    initialY = parent.offsetTop;

    isResizing = true;
    options.onStart?.();

    // 添加 resizing class
    parent.classList.add("resizing");

    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
  }

  function handleMouseMove(e: MouseEvent) {
    if (!isResizing) return;

    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    const dir = options.direction;

    let newWidth = initialWidth;
    let newHeight = initialHeight;
    let newX: number | undefined;
    let newY: number | undefined;

    // 根据方向计算新尺寸
    if (dir.includes("e")) {
      newWidth = Math.max(minWidth, initialWidth + dx);
    }
    if (dir.includes("w")) {
      const w = Math.max(minWidth, initialWidth - dx);
      newX = initialX + (initialWidth - w);
      newWidth = w;
    }
    if (dir.includes("s")) {
      newHeight = Math.max(minHeight, initialHeight + dy);
    }
    if (dir.includes("n")) {
      const h = Math.max(minHeight, initialHeight - dy);
      newY = initialY + (initialHeight - h);
      newHeight = h;
    }

    options.onResize?.({ width: newWidth, height: newHeight, x: newX, y: newY });
  }

  function handleMouseUp() {
    if (!isResizing) return;

    isResizing = false;

    const parent = node.closest(".window") as HTMLElement;
    if (parent) {
      parent.classList.remove("resizing");
    }

    options.onEnd?.();

    window.removeEventListener("mousemove", handleMouseMove);
    window.removeEventListener("mouseup", handleMouseUp);
  }

  node.addEventListener("mousedown", handleMouseDown);

  return {
    update(newOptions: ResizableOptions) {
      options = newOptions;
    },
    destroy() {
      node.removeEventListener("mousedown", handleMouseDown);
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    },
  };
}
