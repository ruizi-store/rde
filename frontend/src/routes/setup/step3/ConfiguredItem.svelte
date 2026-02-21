<script lang="ts">
  const { content, status }: { content: string; status: string } = $props();
</script>

<div class="flex items-center gap-2 rounded-md p-4">
  <span class="text-lg">✅</span>
  <span class="grow">{content}</span>
  <span class="text-success">{status}</span>
</div>

<style>
  div {
    background: rgba(255, 255, 255, 0.8);
  }

  :global(:root[data-theme="dark"]) {
    div {
      background: rgba(255, 255, 255, 0.08);
    }
  }
</style>
