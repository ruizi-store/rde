<script lang="ts">
  interface Props {
    value?: number;
    max?: number;
    size?: "sm" | "md" | "lg";
    variant?: "default" | "success" | "warning" | "error";
    showLabel?: boolean;
    striped?: boolean;
    animated?: boolean;
  }

  let {
    value = 0,
    max = 100,
    size = "md",
    variant = "default",
    showLabel = false,
    striped = false,
    animated = false,
  }: Props = $props();

  let percentage = $derived(Math.min(Math.max((value / max) * 100, 0), 100));
</script>

<div class="progress-wrapper {size}">
  <div
    class="progress-bar"
    class:striped
    class:animated={animated && striped}
    role="progressbar"
    aria-valuenow={value}
    aria-valuemin={0}
    aria-valuemax={max}
  >
    <div class="progress-fill {variant}" style="width: {percentage}%"></div>
  </div>
  {#if showLabel}
    <span class="progress-label">{Math.round(percentage)}%</span>
  {/if}
</div>

<style>
  .progress-wrapper {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;

    &.sm {
      .progress-bar {
        height: 4px;
      }

      .progress-label {
        font-size: 12px;
      }
    }

    &.md {
      .progress-bar {
        height: 8px;
      }

      .progress-label {
        font-size: 14px;
      }
    }

    &.lg {
      .progress-bar {
        height: 12px;
      }

      .progress-label {
        font-size: 16px;
      }
    }
  }

  .progress-bar {
    flex: 1;
    background: var(--bg-tertiary, #e9ecef);
    border-radius: 100px;
    overflow: hidden;

    &.striped .progress-fill {
      background-image: linear-gradient(
        45deg,
        rgba(255, 255, 255, 0.15) 25%,
        transparent 25%,
        transparent 50%,
        rgba(255, 255, 255, 0.15) 50%,
        rgba(255, 255, 255, 0.15) 75%,
        transparent 75%,
        transparent
      );
      background-size: 1rem 1rem;
    }

    &.animated .progress-fill {
      animation: stripes 1s linear infinite;
    }
  }

  .progress-fill {
    height: 100%;
    border-radius: 100px;
    transition: width 0.3s ease;

    &.default {
      background: var(--color-primary, #4a90d9);
    }

    &.success {
      background: #52c41a;
    }

    &.warning {
      background: #faad14;
    }

    &.error {
      background: #ff4d4f;
    }
  }

  .progress-label {
    flex-shrink: 0;
    color: var(--text-secondary, #666);
    min-width: 40px;
    text-align: right;
  }

  @keyframes stripes {
    0% {
      background-position: 1rem 0;
    }
    100% {
      background-position: 0 0;
    }
  }
</style>
