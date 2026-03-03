import type { ArchInfo, BuildTarget } from "./types";

/** 架构信息映射 */
export const ARCH_INFO: Record<string, ArchInfo> = {
  aarch64:     { id: "aarch64",     name: "AArch64 (ARM64)", icon: "mdi:chip",             description: "64-bit ARM architecture" },
  arm:         { id: "arm",         name: "ARM",             icon: "mdi:chip",             description: "32-bit ARM architecture" },
  i386:        { id: "i386",        name: "i386 (x86)",      icon: "mdi:desktop-tower",    description: "32-bit x86 architecture" },
  x86_64:      { id: "x86_64",      name: "x86_64",          icon: "mdi:desktop-tower",    description: "64-bit x86 architecture" },
  riscv32:     { id: "riscv32",     name: "RISC-V 32",       icon: "mdi:memory",           description: "32-bit RISC-V" },
  riscv64:     { id: "riscv64",     name: "RISC-V 64",       icon: "mdi:memory",           description: "64-bit RISC-V" },
  mipsel:      { id: "mipsel",      name: "MIPS (LE)",       icon: "mdi:cpu-32-bit",       description: "MIPS little-endian" },
  mips64el:    { id: "mips64el",    name: "MIPS64 (LE)",     icon: "mdi:cpu-64-bit",       description: "MIPS64 little-endian" },
  loongarch64: { id: "loongarch64", name: "LoongArch64",     icon: "mdi:cpu-64-bit",       description: "64-bit LoongArch" },
  ppc:         { id: "ppc",         name: "PowerPC",         icon: "mdi:server",           description: "32-bit PowerPC" },
  ppc64:       { id: "ppc64",       name: "PowerPC 64",      icon: "mdi:server",           description: "64-bit PowerPC (BE)" },
  ppc64le:     { id: "ppc64le",     name: "PowerPC 64 LE",   icon: "mdi:server",           description: "64-bit PowerPC (LE)" },
  s390x:       { id: "s390x",       name: "s390x",           icon: "mdi:server-network",   description: "IBM Z architecture" },
};

/** 构建目标选项 */
export const BUILD_TARGETS: { id: BuildTarget; name: string; description: string }[] = [
  { id: "kernel-build", name: "内核构建",       description: "编译 Linux 内核" },
  { id: "uboot-build",  name: "U-Boot 构建",   description: "编译 U-Boot 引导程序" },
  { id: "root-build",   name: "根文件系统构建", description: "构建 Buildroot 根文件系统" },
  { id: "all",          name: "完整构建",       description: "构建所有组件（内核 + U-Boot + 根文件系统）" },
];

/** 获取架构显示信息，未知架构返回默认值 */
export function getArchInfo(arch: string): ArchInfo {
  return ARCH_INFO[arch] || {
    id: arch,
    name: arch,
    icon: "mdi:chip",
    description: arch,
  };
}
