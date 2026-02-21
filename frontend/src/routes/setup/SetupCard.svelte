<script lang="ts">
  import { type Snippet } from "svelte";

  interface Props {
    children: Snippet;
    header?: {
      icon: string;
      title: string;
      description: string;
    };
    footer?: Snippet;
    error?: string;
  }

  const { children, header, footer, error }: Props = $props();
</script>

<div class="flex flex-col gap-8">
  {#if header}
    <header class="text-center">
      <div class="step-icon">{header.icon}</div>
      <h1 class="step-title">{header.title}</h1>
      <p class="step-description">{header.description}</p>
    </header>
  {/if}

  {@render children()}

  {#if error}
    <div class="error-box">
      <p>{error}</p>
    </div>
  {/if}

  {#if footer}
    <footer class="step-footer">
      {@render footer()}
    </footer>
  {/if}
</div>

<style>
  .step-icon {
    font-size: 48px;
    margin-bottom: 16px;
  }

  .step-title {
    font-size: 28px;
    font-weight: 600;
    margin: 0 0 8px 0;
  }

  .step-description {
    font-size: 16px;
    color: var(--text-secondary);
    margin: 0;
  }

  .step-footer {
    display: flex;
    padding-top: 16px;
    border-top: 1px solid rgba(0, 0, 0, 0.05);

    :global(*:last-child) {
      margin-inline-start: auto;
    }
  }

  .error-box {
    background: rgba(239, 68, 68, 0.05);
    border: 1px solid rgba(239, 68, 68, 0.2);
    border-radius: 12px;
    padding: 16px;
    text-align: center;
  }

  .error-box p {
    color: #ef4444;
    margin: 0;
  }
</style>
