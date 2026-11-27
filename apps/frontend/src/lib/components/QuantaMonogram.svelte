<script lang="ts">
  let { size = 28 }: { size?: number } = $props();
</script>

<svg
  class="quanta-monogram-svg"
  width={size}
  height={size}
  viewBox="0 0 32 32"
  fill="none"
  xmlns="http://www.w3.org/2000/svg"
  aria-hidden="true"
>
  <rect width="32" height="32" rx="4" class="monogram-frame" stroke-width="1" />
  <circle cx="15" cy="15" r="7" class="monogram-ring" stroke-width="1.8" />
  <path d="M19.5 19.5L24.5 24.5" class="monogram-vector" stroke-width="2.2" stroke-linecap="round" />
  <circle cx="15" cy="15" r="2" class="monogram-node" />
</svg>

<style>
  .quanta-monogram-svg {
    display: inline-block;
    flex-shrink: 0;
  }

  .monogram-frame {
    fill: #0f172a;
    stroke: #0f172a;
  }

  .monogram-ring {
    stroke: #f8f9fa;
  }

  .monogram-vector {
    stroke: #2d9f93;
  }

  .monogram-node {
    fill: #2d9f93;
  }

  :global([data-theme="dark"]) .monogram-frame {
    fill: #171b22;
    stroke: rgba(255, 255, 255, 0.14);
  }

  :global([data-theme="dark"]) .monogram-ring {
    stroke: #f4f5f6;
  }

  :global([data-theme="dark"]) .monogram-vector {
    stroke: #2d9f93;
  }

  :global([data-theme="dark"]) .monogram-node {
    fill: #2d9f93;
  }
</style>
