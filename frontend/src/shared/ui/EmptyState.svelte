<script lang="ts">
  import Icon from "@iconify/svelte";

  interface Props {
    icon?: string;
    title: string;
    description?: string;
    actionLabel?: string;
    onaction?: () => void;
  }

  let { icon = "mdi:inbox-outline", title, description, actionLabel, onaction }: Props = $props();
</script>

<div class="empty-state">
  <div class="icon">
    <Icon {icon} width="48" height="48" />
  </div>
  <h3 class="title">{title}</h3>
  {#if description}
    <p class="description">{description}</p>
  {/if}
  {#if actionLabel && onaction}
    <button class="action-btn" onclick={onaction}>
      {actionLabel}
    </button>
  {/if}
</div>

<style>
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 24px;
    text-align: center;
  }

  .icon {
    color: #9ca3af;
    margin-bottom: 16px;
  }

  .title {
    font-size: 16px;
    font-weight: 500;
    color: var(--text-primary, #333);
    margin: 0 0 8px 0;
  }

  .description {
    font-size: 14px;
    color: var(--text-secondary, #666);
    margin: 0 0 16px 0;
    max-width: 320px;
  }

  .action-btn {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 8px 16px;
    background: var(--color-primary, #0066cc);
    color: white;
    border: none;
    border-radius: 6px;
    font-size: 14px;
    cursor: pointer;
    transition: opacity 0.15s;

    &:hover {
      opacity: 0.9;
    }
  }
</style>
