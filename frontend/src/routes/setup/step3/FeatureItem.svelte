<script lang="ts">
  const { icon, label }: { icon: string; label: string } = $props();
</script>

<div class="flex flex-col items-center gap-4 rounded-md p-4">
  <span class="text-3xl">{icon}</span>
  <span class="text-sm">{label}</span>
</div>

<style>
  div {
    background: rgba(255, 255, 255, 0.5);
    transition: all 0.2s ease;
  }

  div:hover {
    background: rgba(255, 255, 255, 0.8);
    transform: translateY(-2px);
  }

  :global(:root[data-theme="dark"]) {
    div {
      background: rgba(255, 255, 255, 0.05);
    }

    div:hover {
      background: rgba(255, 255, 255, 0.1);
    }
  }
</style>
