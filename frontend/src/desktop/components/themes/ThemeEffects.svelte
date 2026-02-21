<!-- ThemeEffects.svelte - 主题特效容器 -->
<script lang="ts">
  import NotificationBubbleManager from "./aquarium/NotificationBubbleManager.svelte";

  // 主题类型
  type ThemeType = "none" | "aquarium" | "space" | "forest";

  interface Props {
    theme?: ThemeType;
    taskbarHeight?: number;
  }

  let { theme = "aquarium", taskbarHeight = 48 }: Props = $props();
</script>

<div class="theme-effects">
  <!-- 通知泡泡 - 由 notificationBubbleStore 控制 -->
  <NotificationBubbleManager {taskbarHeight} />

  <!-- 未来可以添加更多主题特效 -->
  <!-- {#if theme === "aquarium"}
    <AquariumEffects />
  {/if} -->

  <!-- {#if theme === "space"}
    <StarField />
  {/if} -->

  <!-- {#if theme === "forest"}
    <FallingLeaves />
  {/if} -->
</div>

<style>
  .theme-effects {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 2;
    overflow: hidden;
  }
</style>
