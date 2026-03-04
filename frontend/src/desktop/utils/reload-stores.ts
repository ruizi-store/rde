/**
 * 为当前用户重新加载所有个性化 store
 * 在登录后或页面加载时调用，确保各 store 使用正确的用户专属数据
 */
export function reloadAllStoresForUser(): void {
  // 动态导入避免循环依赖
  import("$shared/stores/theme.svelte").then(({ theme }) => theme.reloadForUser());
  import("$shared/stores/settings.svelte").then(({ settings }) => settings.reloadForUser());
  import("$desktop/stores/desktop.svelte").then(({ desktop }) => desktop.reloadForUser());
  import("$desktop/stores/apps.svelte").then(({ apps }) => apps.reloadForUser());
}
