<script lang="ts">
  let { size = 32 }: { size?: number } = $props();
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
  <!-- Institutional Frame -->
  <rect width="32" height="32" rx="6" class="monogram-frame" stroke-width="1.2" />
  <!-- Capital Ring with Quadrant Geometry -->
  <circle cx="15" cy="15" r="7.5" class="monogram-ring" stroke-width="1.75" />
  <line x1="15" y1="4.5" x2="15" y2="7.5" class="monogram-axis" stroke-width="1.5" stroke-linecap="round" />
  <line x1="4.5" y1="15" x2="7.5" y2="15" class="monogram-axis-secondary" stroke-width="1.25" stroke-linecap="round" />
  <!-- Gold Alpha Vector / Q Terminal -->
  <path d="M19.5 19.5L25 25" class="monogram-vector" stroke-width="2.5" stroke-linecap="round" />
  <!-- Center Core Node -->
  <circle cx="15" cy="15" r="2.25" class="monogram-node" />
</svg>

<style>
  .quanta-monogram-svg {
    display: inline-block;
    flex-shrink: 0;
  }

  .monogram-frame {
    fill: #0F172A;
    stroke: #253347;
  }

  .monogram-ring {
    stroke: #94A3B8;
  }

  .monogram-axis {
    stroke: #D4AF37;
  }

  .monogram-axis-secondary {
    stroke: #64748B;
  }

  .monogram-vector {
    stroke: #D4AF37;
  }

  .monogram-node {
    fill: #D4AF37;
  }

  :global([data-theme="light"]) .monogram-frame {
    fill: #F1F5F9;
    stroke: #CBD5E1;
  }

  :global([data-theme="light"]) .monogram-ring {
    stroke: #475569;
  }

  :global([data-theme="light"]) .monogram-axis {
    stroke: #B8860B;
  }

  :global([data-theme="light"]) .monogram-axis-secondary {
    stroke: #94A3B8;
  }

  :global([data-theme="light"]) .monogram-vector {
    stroke: #B8860B;
  }

  :global([data-theme="light"]) .monogram-node {
    fill: #B8860B;
  }
</style>
