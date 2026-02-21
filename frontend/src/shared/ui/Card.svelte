<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    title?: string;
    subtitle?: string;
    hoverable?: boolean;
    bordered?: boolean;
    padding?: "none" | "sm" | "md" | "lg";
    header?: Snippet;
    footer?: Snippet;
    actions?: Snippet;
    children: Snippet;
  }

  let {
    title = "",
    subtitle = "",
    hoverable = false,
    bordered = true,
    padding = "md",
    header,
    footer,
    actions,
    children,
  }: Props = $props();
</script>

<div class="card padding-{padding}" class:hoverable class:bordered>
  {#if header || title || subtitle || actions}
    <div class="card-header">
      {#if header}
        {@render header()}
      {:else}
        <div class="header-content">
          {#if title}
            <h3 class="card-title">{title}</h3>
          {/if}
          {#if subtitle}
            <p class="card-subtitle">{subtitle}</p>
          {/if}
        </div>
      {/if}
      {#if actions}
        <div class="card-actions">
          {@render actions()}
        </div>
      {/if}
    </div>
  {/if}

  <div class="card-body">
    {@render children()}
  </div>

  {#if footer}
    <div class="card-footer">
      {@render footer()}
    </div>
  {/if}
</div>

<style>
  .card {
    background: var(--bg-card, white);
    border-radius: 12px;
    transition:
      box-shadow 0.2s,
      transform 0.2s;

    &.bordered {
      border: 1px solid var(--border-color, #e0e0e0);
    }

    &:not(.bordered) {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    }

    &.hoverable {
      cursor: pointer;

      &:hover {
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
        transform: translateY(-2px);
      }
    }

    &.padding-none {
      .card-body {
        padding: 0;
      }
    }

    &.padding-sm {
      .card-header,
      .card-body,
      .card-footer {
        padding: 12px;
      }
    }

    &.padding-md {
      .card-header,
      .card-body,
      .card-footer {
        padding: 16px;
      }
    }

    &.padding-lg {
      .card-header,
      .card-body,
      .card-footer {
        padding: 24px;
      }
    }
  }

  .card-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .header-content {
    flex: 1;
    min-width: 0;
  }

  .card-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .card-subtitle {
    margin: 4px 0 0;
    font-size: 14px;
    color: var(--text-muted, #888);
  }

  .card-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .card-footer {
    border-top: 1px solid var(--border-color, #e0e0e0);
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
