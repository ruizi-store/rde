// Linux Lab 类型定义（容器模式）

/** 开发板信息 */
export interface Board {
  arch: string;
  name: string;
  full_path: string;
  cpu: string;
  mem: string;
  smp: number;
  linux: string;
  qemu: string;
  uboot: string;
  buildroot: string;
  netdev: string;
  serial: string;
  rootdev: string;
}

/** 实验环境状态（容器模式） */
export interface LabStatus {
  docker_ok: boolean;
  image_ready: boolean;
  container_running: boolean;
  container_exists: boolean;
  current_board: string;
  building: boolean;
  booting: boolean;
  image: string;
}

/** SSE 进度事件 */
export interface ProgressEvent {
  status: "running" | "completed" | "failed";
  message?: string;
  line?: string;
}

/** 构建目标 */
export type BuildTarget = "kernel" | "kernel-build" | "modules" | "modules-install" | "uboot" | "uboot-build" | "root" | "root-build" | "all";

/** 构建状态 */
export interface BuildStatus {
  building: boolean;
  running: boolean;
}

/** 架构信息 */
export interface ArchInfo {
  id: string;
  name: string;
  icon: string;
  description: string;
}
