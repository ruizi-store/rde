<script lang="ts">
  interface Props {
    size?: "sm" | "md" | "lg" | "xl";
    color?: string;
    thickness?: number;
    center?: boolean;
  }

  let { size = "md", color = "", thickness = 2, center = false }: Props = $props();

  const sizes = {
    sm: 16,
    md: 24,
    lg: 32,
    xl: 48,
  };
</script>

<div class="spinner-wrapper" class:center>
  <svg
    class="spinner"
    width={sizes[size]}
    height={sizes[size]}
    viewBox="0 0 24 24"
    style="--spinner-color: {color || 'var(--color-primary, #4a90d9)'}; --thickness: {thickness}"
  >
    <circle class="spinner-track" cx="12" cy="12" r="10" fill="none" stroke-width={thickness} />
    <circle
      class="spinner-arc"
      cx="12"
      cy="12"
      r="10"
      fill="none"
      stroke-width={thickness}
      stroke-linecap="round"
    />
  </svg>
</div>

<style>
  .spinner-wrapper {
    display: inline-flex;
    align-items: center;
    justify-content: center;

    &.center {
      width: 100%;
      height: 100%;
      min-height: 100px;
    }
  }

  .spinner {
    animation: rotate 1s linear infinite;
  }

  .spinner-track {
    stroke: rgba(0, 0, 0, 0.1);
  }

  .spinner-arc {
    stroke: var(--spinner-color);
    stroke-dasharray: 45, 200;
    stroke-dashoffset: 0;
    animation: dash 1.5s ease-in-out infinite;
  }

  @keyframes rotate {
    100% {
      transform: rotate(360deg);
    }
  }

  @keyframes dash {
    0% {
      stroke-dasharray: 1, 200;
      stroke-dashoffset: 0;
    }
    50% {
      stroke-dasharray: 45, 200;
      stroke-dashoffset: -15;
    }
    100% {
      stroke-dasharray: 45, 200;
      stroke-dashoffset: -62;
    }
  }
</style>
