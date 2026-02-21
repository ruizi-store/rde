<script lang="ts">
  interface Props {
    src?: string;
    alt?: string;
    size?: "xs" | "sm" | "md" | "lg" | "xl" | number;
    name?: string;
    status?: "online" | "offline" | "busy" | "away" | "";
    shape?: "circle" | "square";
  }

  let {
    src = "",
    alt = "",
    size = "md",
    name = "",
    status = "",
    shape = "circle",
  }: Props = $props();

  const sizes = {
    xs: 24,
    sm: 32,
    md: 40,
    lg: 56,
    xl: 80,
  };

  let computedSize = $derived(typeof size === "number" ? size : sizes[size]);
  let fontSize = $derived(computedSize * 0.4);

  /* 根据名字生成颜色 */
  function getColorFromName(name: string): string {
    const colors = [
      "#f56a00",
      "#7265e6",
      "#ffbf00",
      "#00a2ae",
      "#1890ff",
      "#52c41a",
      "#eb2f96",
      "#722ed1",
    ];
    let hash = 0;
    for (let i = 0; i < name.length; i++) {
      hash = name.charCodeAt(i) + ((hash << 5) - hash);
    }
    return colors[Math.abs(hash) % colors.length];
  }

  /* 获取名字缩写 */
  function getInitials(name: string): string {
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
    }
    return name.slice(0, 2).toUpperCase();
  }

  let bgColor = $derived(name ? getColorFromName(name) : "#e0e0e0");
  let initials = $derived(name ? getInitials(name) : "");
  let imageError = $state(false);
</script>

<div
  class="avatar"
  class:square={shape === "square"}
  style="
    width: {computedSize}px;
    height: {computedSize}px;
    font-size: {fontSize}px;
    background-color: {!src || imageError ? bgColor : 'transparent'};
  "
>
  {#if src && !imageError}
    <img {src} {alt} onerror={() => (imageError = true)} />
  {:else if name}
    <span class="initials">{initials}</span>
  {:else}
    <svg viewBox="0 0 24 24" fill="currentColor" class="default-icon">
      <path
        d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"
      />
    </svg>
  {/if}

  {#if status}
    <span class="status-indicator status-{status}"></span>
  {/if}
</div>

<style>
  .avatar {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    overflow: hidden;
    flex-shrink: 0;
    color: white;

    &.square {
      border-radius: 8px;
    }

    img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
  }

  .initials {
    font-weight: 500;
    user-select: none;
  }

  .default-icon {
    width: 60%;
    height: 60%;
    color: rgba(255, 255, 255, 0.8);
  }

  .status-indicator {
    position: absolute;
    bottom: 0;
    right: 0;
    width: 25%;
    height: 25%;
    min-width: 8px;
    min-height: 8px;
    border-radius: 50%;
    border: 2px solid white;
    box-sizing: content-box;
  }

  .status-online {
    background: #52c41a;
  }

  .status-offline {
    background: #8c8c8c;
  }

  .status-busy {
    background: #ff4d4f;
  }

  .status-away {
    background: #faad14;
  }
</style>
