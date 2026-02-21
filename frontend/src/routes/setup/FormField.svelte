<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    icon?: string;
    label: string;
    labelFor: string;
    hint?: string;
    children: Snippet;
  }

  const { icon, label, labelFor, hint, children }: Props = $props();
</script>

<fieldset class="flex flex-col gap-2">
  <label class="font-medium" for={labelFor}>
    {#if icon}
      <span class="me-1 text-lg">{icon}</span>
    {/if}
    {label}
  </label>
  {@render children()}
  {#if hint}
    <div class="text-secondary text-sm">{hint}</div>
  {/if}
</fieldset>
