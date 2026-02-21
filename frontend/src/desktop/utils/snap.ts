// 窗口吸附工具
// 实现窗口拖拽到屏幕边缘时的吸附效果

export interface SnapZone {
  id:
    | "left"
    | "right"
    | "top"
    | "top-left"
    | "top-right"
    | "bottom-left"
    | "bottom-right"
    | "maximize";
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface SnapPreview {
  visible: boolean;
  x: number;
  y: number;
  width: number;
  height: number;
}

// 吸附区域配置
const EDGE_THRESHOLD = 20; // 边缘检测阈值 (像素)
const CORNER_SIZE = 100; // 角落检测区域大小

// 获取吸附区域
export function getSnapZone(
  mouseX: number,
  mouseY: number,
  screenWidth: number,
  screenHeight: number,
  taskbarHeight: number = 48,
): SnapZone | null {
  const availableHeight = screenHeight - taskbarHeight;
  const halfWidth = screenWidth / 2;
  const halfHeight = availableHeight / 2;

  // 顶部边缘 - 最大化
  if (mouseY <= EDGE_THRESHOLD) {
    // 左上角
    if (mouseX <= CORNER_SIZE) {
      return {
        id: "top-left",
        x: 0,
        y: 0,
        width: halfWidth,
        height: halfHeight,
      };
    }
    // 右上角
    if (mouseX >= screenWidth - CORNER_SIZE) {
      return {
        id: "top-right",
        x: halfWidth,
        y: 0,
        width: halfWidth,
        height: halfHeight,
      };
    }
    // 顶部中间 - 最大化
    return {
      id: "maximize",
      x: 0,
      y: 0,
      width: screenWidth,
      height: availableHeight,
    };
  }

  // 左边缘
  if (mouseX <= EDGE_THRESHOLD) {
    // 左下角
    if (mouseY >= screenHeight - CORNER_SIZE - taskbarHeight) {
      return {
        id: "bottom-left",
        x: 0,
        y: halfHeight,
        width: halfWidth,
        height: halfHeight,
      };
    }
    // 左半屏
    return {
      id: "left",
      x: 0,
      y: 0,
      width: halfWidth,
      height: availableHeight,
    };
  }

  // 右边缘
  if (mouseX >= screenWidth - EDGE_THRESHOLD) {
    // 右下角
    if (mouseY >= screenHeight - CORNER_SIZE - taskbarHeight) {
      return {
        id: "bottom-right",
        x: halfWidth,
        y: halfHeight,
        width: halfWidth,
        height: halfHeight,
      };
    }
    // 右半屏
    return {
      id: "right",
      x: halfWidth,
      y: 0,
      width: halfWidth,
      height: availableHeight,
    };
  }

  return null;
}

// 创建吸附预览状态
export function createSnapPreviewState() {
  let preview = $state<SnapPreview>({
    visible: false,
    x: 0,
    y: 0,
    width: 0,
    height: 0,
  });

  return {
    get value() {
      return preview;
    },
    show(zone: SnapZone) {
      preview = {
        visible: true,
        x: zone.x,
        y: zone.y,
        width: zone.width,
        height: zone.height,
      };
    },
    hide() {
      preview = { ...preview, visible: false };
    },
  };
}
