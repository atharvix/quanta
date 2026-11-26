<script lang="ts">
  import type { SystemHealthResponse } from '$lib/types';

  let {
    status = {
      gateway: 'online',
      database: 'up',
      aiEngine: 'up',
      optimizer: 'down'
    }
  }: {
    status?: {
      gateway: 'online' | 'offline';
      database: 'up' | 'down' | 'checking';
      aiEngine: 'up' | 'down' | 'checking';
      optimizer: 'up' | 'down' | 'checking';
    };
  } = $props();
</script>

<div class="architecture-visual-root" aria-label="QUANTA Treasury Architecture Visual">
  <!-- Subtle Blueprint Grid Backdrop (Pure SVG) -->
  <div class="grid-backdrop" aria-hidden="true">
    <svg class="grid-svg" width="100%" height="100%" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <pattern id="arch-grid-pattern" width="36" height="36" patternUnits="userSpaceOnUse">
          <path d="M 36 0 L 0 0 0 36" fill="none" stroke="currentColor" stroke-width="0.75" />
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#arch-grid-pattern)" />
    </svg>
  </div>

  <div class="visual-inner">
    <!-- Header Badge -->
    <div class="arch-meta-header">
      <div class="meta-left">
        <span class="meta-dot"></span>
        <span class="meta-title">TREASURY ARCHITECTURE &amp; GOVERNANCE</span>
      </div>
      <span class="meta-tag">ENTERPRISE SYSTEM</span>
    </div>

    <!-- Geometric Financial System Architecture Diagram (Pure SVG) -->
    <div class="system-diagram-container">
      <svg
        class="system-diagram-svg"
        viewBox="0 0 460 280"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        aria-hidden="true"
      >
        <!-- Structural Connection Lines -->
        <g class="connector-lines" stroke="currentColor" stroke-width="1.25" stroke-dasharray="3 3">
          <!-- Horizontal axis -->
          <line x1="40" y1="140" x2="420" y2="140" />
          <!-- Diagonal cross-connectors -->
          <line x1="80" y1="60" x2="230" y2="140" />
          <line x1="80" y1="220" x2="230" y2="140" />
          <line x1="380" y1="60" x2="230" y2="140" />
          <line x1="380" y1="220" x2="230" y2="140" />
          <!-- Vertical balance axes -->
          <line x1="80" y1="60" x2="80" y2="220" />
          <line x1="380" y1="60" x2="380" y2="220" />
        </g>

        <!-- Node 1: Capital Inflow / Assets (Upper Left) -->
        <g class="diagram-node node-assets" transform="translate(80, 60)">
          <circle r="22" class="node-bg" />
          <circle r="22" class="node-ring" stroke="currentColor" stroke-width="1.5" />
          <!-- Vault / Capital Icon -->
          <path
            d="M -7 -4 h 14 v 10 h -14 z M -4 -4 v -4 a 4 4 0 0 1 8 0 v 4"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linejoin="round"
          />
          <text y="36" class="node-label" text-anchor="middle">ASSETS</text>
          <text y="48" class="node-sublabel" text-anchor="middle">Sovereign &amp; Equity</text>
        </g>

        <!-- Node 2: Obligations / Liabilities (Lower Left) -->
        <g class="diagram-node node-liabilities" transform="translate(80, 220)">
          <circle r="22" class="node-bg" />
          <circle r="22" class="node-ring" stroke="currentColor" stroke-width="1.5" />
          <!-- Scale / Debt Icon -->
          <path
            d="M 0 -8 v 16 M -6 -4 l 6 -4 6 4 M -9 4 l 3 -8 3 8 h -6 M 3 4 l 3 -8 3 8 h -6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            stroke-linecap="round"
          />
          <text y="-32" class="node-label" text-anchor="middle">LIABILITIES</text>
          <text y="-20" class="node-sublabel" text-anchor="middle">Credit Facilities</text>
        </g>

        <!-- CENTRAL CORE NODE: QUANTA Autonomous Allocation Engine -->
        <g class="diagram-node node-core" transform="translate(230, 140)">
          <!-- Outer Pulsing Ring -->
          <circle r="44" class="core-outer-ring" stroke="currentColor" stroke-width="1" stroke-dasharray="4 2" />
          <circle r="34" class="core-bg" />
          <circle r="34" class="core-border" stroke="currentColor" stroke-width="2" />
          <!-- QUANTA Q Geometry Icon -->
          <circle cx="-1" cy="-1" r="12" fill="none" stroke="currentColor" stroke-width="2" />
          <line x1="7" y1="7" x2="15" y2="15" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
          <text y="54" class="node-label-core" text-anchor="middle">QUANTA CORE</text>
          <text y="66" class="node-sublabel" text-anchor="middle">Autonomous Orchestration</text>
        </g>

        <!-- Node 3: Liquidity Floor & Reserve (Upper Right) -->
        <g class="diagram-node node-liquidity" transform="translate(380, 60)">
          <circle r="22" class="node-bg" />
          <circle r="22" class="node-ring" stroke="currentColor" stroke-width="1.5" />
          <!-- Droplet / Liquidity Icon -->
          <path
            d="M 0 -8 C -5 -2 -7 2 -7 5 A 7 7 0 0 0 7 5 C 7 2 5 -2 0 -8 Z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
          />
          <text y="36" class="node-label" text-anchor="middle">LIQUIDITY</text>
          <text y="48" class="node-sublabel" text-anchor="middle">Mandatory Floor</text>
        </g>

        <!-- Node 4: Risk Analytics & Governance (Lower Right) -->
        <g class="diagram-node node-risk" transform="translate(380, 220)">
          <circle r="22" class="node-bg" />
          <circle r="22" class="node-ring" stroke="currentColor" stroke-width="1.5" />
          <!-- Shield / Risk Icon -->
          <path
            d="M 0 -8 L 7 -4 V 1 C 7 6 3 9 0 11 C -3 9 -7 6 -7 1 V -4 Z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
          />
          <text y="-32" class="node-label" text-anchor="middle">GOVERNANCE</text>
          <text y="-20" class="node-sublabel" text-anchor="middle">Bayesian Constraints</text>
        </g>
      </svg>
    </div>

    <!-- Core Value Propositions / Architecture Pillars -->
    <div class="pillars-grid">
      <div class="pillar-card">
        <div class="pillar-header">
          <span class="pillar-bullet"></span>
          <span class="pillar-title">Unified Balance Sheet</span>
        </div>
        <p class="pillar-desc">
          Continuous consolidation across corporate portfolios, senior debt facilities, and liquidity tiers.
        </p>
      </div>

      <div class="pillar-card">
        <div class="pillar-header">
          <span class="pillar-bullet"></span>
          <span class="pillar-title">Autonomous Allocation</span>
        </div>
        <p class="pillar-desc">
          Neuro-symbolic search algorithms optimize returns bounded by strict liquidity floors and risk constraints.
        </p>
      </div>

      <div class="pillar-card">
        <div class="pillar-header">
          <span class="pillar-bullet"></span>
          <span class="pillar-title">Institutional Oversight</span>
        </div>
        <p class="pillar-desc">
          Advisory lock transaction isolation, multi-role RBAC enforcement, and cryptographically sealed audit logs.
        </p>
      </div>
    </div>
  </div>
</div>

<style>
  .architecture-visual-root {
    position: relative;
    width: 100%;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background-color: var(--color-surface);
    overflow: hidden;
    box-sizing: border-box;
  }

  /* SVG Blueprint Grid Backdrop */
  .grid-backdrop {
    position: absolute;
    inset: 0;
    opacity: 0.18;
    color: var(--color-text-secondary);
    pointer-events: none;
  }

  .grid-svg {
    width: 100%;
    height: 100%;
  }

  .visual-inner {
    position: relative;
    z-index: 1;
    padding: 1.5rem 1.75rem;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  /* Header Badge */
  .arch-meta-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .meta-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .meta-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-primary);
  }

  .meta-title {
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--color-text-secondary);
    text-transform: uppercase;
  }

  .meta-tag {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.06em;
  }

  /* System Diagram SVG */
  .system-diagram-container {
    width: 100%;
    display: flex;
    justify-content: center;
    align-items: center;
  }

  .system-diagram-svg {
    width: 100%;
    max-width: 440px;
    height: auto;
    overflow: visible;
  }

  .connector-lines {
    color: var(--color-border);
  }

  .diagram-node {
    cursor: default;
    transition: transform 0.2s ease;
  }

  .node-bg {
    fill: var(--color-surface-elevated, var(--color-surface));
  }

  .node-assets {
    color: #315fea;
  }

  .node-liabilities {
    color: #64748b;
  }

  .node-liquidity {
    color: #059669;
  }

  .node-risk {
    color: #d97706;
  }

  .node-core {
    color: var(--color-primary);
  }

  .core-bg {
    fill: var(--color-surface);
  }

  .core-outer-ring {
    opacity: 0.6;
  }

  .node-label {
    font-size: 8.5px;
    font-weight: 700;
    fill: var(--color-text);
    letter-spacing: 0.06em;
  }

  .node-label-core {
    font-size: 9.5px;
    font-weight: 800;
    fill: var(--color-text);
    letter-spacing: 0.08em;
  }

  .node-sublabel {
    font-size: 7.5px;
    font-weight: 500;
    fill: var(--color-text-muted);
  }

  /* Pillars Grid */
  .pillars-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.875rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--color-border-subtle);
  }

  .pillar-card {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .pillar-header {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .pillar-bullet {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background-color: var(--color-primary);
    flex-shrink: 0;
  }

  .pillar-title {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .pillar-desc {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    line-height: 1.4;
    margin: 0;
  }

  @media (max-width: 640px) {
    .pillars-grid {
      grid-template-columns: 1fr;
      gap: 0.75rem;
    }
  }
</style>
