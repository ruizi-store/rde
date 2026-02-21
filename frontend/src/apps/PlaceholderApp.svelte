<script lang="ts">
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";

  let { windowId }: { windowId?: string } = $props();
</script>

<div class="placeholder-app">
  <Icon icon="mdi:wrench" width="64" />
  <h2>{$t("common.underDevelopment")}</h2>
  <p>{$t("common.underDevelopmentDesc")}</p>
</div>

<style>
  .placeholder-app {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    background: #fafafa;
    color: #999;
    text-align: center;
    padding: 40px;

    h2 {
      margin: 24px 0 12px;
      font-size: 20px;
      font-weight: 500;
      color: #666;
    }

    p {
      font-size: 14px;
    }
  }
</style>
