// 用户头像工具
// 优先显示用户上传头像，否则生成默认虚拟头像

/**
 * DiceBear 风格配置
 * adventurer - 卡通冒险家风格
 * avataaars - 3D 卡通人物风格（类似 Notion / Slack）
 * lorelei - 线条肖像（艺术风）
 * notionists - Notion 风格人物
 * bottts - 机器人风格
 */
const AVATAR_STYLE = "adventurer";

/**
 * 生成基于用户名的默认头像 URL（DiceBear API）
 * 每个用户名会生成唯一且确定性的卡通虚拟人物
 */
export function getDefaultAvatarUrl(seed: string): string {
  const encoded = encodeURIComponent(seed);
  return `https://api.dicebear.com/9.x/${AVATAR_STYLE}/svg?seed=${encoded}&backgroundColor=b6e3f4,c0aede,d1d4f9,ffd5dc,ffdfbf`;
}

/**
 * 获取用户头像 URL（优先上传头像，否则使用本地生成头像）
 */
export function getAvatarUrl(user: { avatar?: string; username?: string } | null | undefined): string {
  if (user?.avatar) {
    return user.avatar;
  }
  // 使用本地生成头像，避免依赖外部 DiceBear API（可能不可达）
  return getLocalAvatarSvg(user?.username || "user");
}

/**
 * 纯本地生成 SVG 头像（离线备选方案）
 * 基于用户名哈希生成渐变背景 + 首字母
 */
export function getLocalAvatarSvg(username: string): string {
  const hash = simpleHash(username);
  const hue1 = hash % 360;
  const hue2 = (hash * 7 + 120) % 360;
  const initial = (username[0] || "U").toUpperCase();

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
    <defs>
      <linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
        <stop offset="0%" stop-color="hsl(${hue1}, 70%, 60%)" />
        <stop offset="100%" stop-color="hsl(${hue2}, 70%, 50%)" />
      </linearGradient>
    </defs>
    <rect width="64" height="64" rx="32" fill="url(#g)" />
    <text x="32" y="32" dy=".35em" text-anchor="middle"
      font-family="system-ui, sans-serif" font-size="28" font-weight="600"
      fill="white">${initial}</text>
  </svg>`;

  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function simpleHash(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}
