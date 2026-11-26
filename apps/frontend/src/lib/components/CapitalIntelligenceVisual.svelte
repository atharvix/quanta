<script lang="ts">
  import { onMount } from 'svelte';

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

  type StreamKey = 'assets' | 'liabilities' | 'liquidity' | 'risk' | 'solvers' | 'governance';

  interface StreamMeta {
    key: StreamKey;
    label: string;
    sublabel: string;
    code: string;
    domain: string;
    mechanism: string;
    operationalRule: string;
    accentColor: string;
    yStart: number;
    pathD: string;
    markerPos: { x: number; y: number };
  }

  const streams: Record<StreamKey, StreamMeta> = {
    assets: {
      key: 'assets',
      label: 'Asset Reserves',
      sublabel: 'Sovereign & Liquid Assets',
      code: 'STR-01',
      domain: 'Multi-Asset Balance Sheet Allocation',
      mechanism: 'Dynamic yield allocation across sovereign paper, commercial debt, and tier-1 cash equivalents.',
      operationalRule: 'Maximizes risk-adjusted yield while enforcing strict liquidation horizon minimums.',
      accentColor: '#d4af37',
      yStart: 35,
      pathD: 'M 30,35 L 160,35 C 260,35 360,102 472,105',
      markerPos: { x: 300, y: 72 }
    },
    liabilities: {
      key: 'liabilities',
      label: 'Debt Service',
      sublabel: 'Credit Commitments & Amortization',
      code: 'STR-02',
      domain: 'Senior Facilities & Debt Servicing',
      mechanism: 'Deterministic repayment scheduling, interest-rate hedging, and covenant headroom monitoring.',
      operationalRule: 'Mandates reserve coverage for near-term debt service obligations prior to allocation.',
      accentColor: '#94a3b8',
      yStart: 70,
      pathD: 'M 30,70 L 160,70 C 260,70 360,111 472,112',
      markerPos: { x: 290, y: 92 }
    },
    liquidity: {
      key: 'liquidity',
      label: 'Liquidity Floor',
      sublabel: 'Basel III 30-Day Preservation',
      code: 'STR-03',
      domain: 'Intraday Liquidity Coverage (LCR)',
      mechanism: 'Automated reserve floors ensuring instant fulfillment of intraday operational obligations.',
      operationalRule: 'Hard stop threshold prohibits capital deployment that violates the corporate safety floor.',
      accentColor: '#38bdf8',
      yStart: 105,
      pathD: 'M 30,105 L 160,105 C 260,105 360,118 472,118',
      markerPos: { x: 275, y: 112 }
    },
    risk: {
      key: 'risk',
      label: 'Bayesian Risk',
      sublabel: 'Macro Stress & Volatility Filters',
      code: 'STR-04',
      domain: 'Dynamic Macroeconomic Belief Network',
      mechanism: 'Posterior belief updating across inflation shocks, interest rate shifts, and market volatility.',
      operationalRule: 'Stressed liquidity multipliers dynamically restrict risk-weighted allocations.',
      accentColor: '#f59e0b',
      yStart: 140,
      pathD: 'M 30,140 L 160,140 C 260,140 360,126 472,124',
      markerPos: { x: 280, y: 133 }
    },
    solvers: {
      key: 'solvers',
      label: 'Allocation Solvers',
      sublabel: 'A* Admissible & Quadratic Solvers',
      code: 'STR-05',
      domain: 'Pareto-Optimal Allocation Engine',
      mechanism: 'Bounded admissible search, convex quadratic solvers, and branch-and-bound optimization.',
      operationalRule: 'Deterministic polynomial-time guarantees bounded solver execution latency.',
      accentColor: '#10b981',
      yStart: 175,
      pathD: 'M 30,175 L 160,175 C 260,175 360,133 472,130',
      markerPos: { x: 295, y: 153 }
    },
    governance: {
      key: 'governance',
      label: 'Governance & Audit',
      sublabel: 'Advisory Locks & Immutable Ledger',
      code: 'STR-06',
      domain: 'Concurrency Control & Integrity',
      mechanism: 'Transactional advisory locks preventing race conditions with an immutable audit journal.',
      operationalRule: 'Strict transaction isolation ensures zero conflicting rebalances across operators.',
      accentColor: '#cbd5e1',
      yStart: 210,
      pathD: 'M 30,210 L 160,210 C 260,210 360,142 472,138',
      markerPos: { x: 310, y: 172 }
    }
  };

  let activeKey = $state<StreamKey | 'core'>('assets');
  let prefersReducedMotion = $state(false);

  onMount(() => {
    if (typeof window !== 'undefined') {
      const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
      prefersReducedMotion = mediaQuery.matches;
      const handler = (e: MediaQueryListEvent) => {
        prefersReducedMotion = e.matches;
      };
      mediaQuery.addEventListener('change', handler);
      return () => mediaQuery.removeEventListener('change', handler);
    }
  });

  const activeStream = $derived(activeKey === 'core' ? null : streams[activeKey]);
</script>

<div
  class="capital-streams-root"
  aria-label="QUANTA Interactive Capital Intelligence System"
  role="region"
>
  <!-- Stream Stage Header -->
  <div class="streams-header">
    <div class="header-left">
      <span class="status-marker"></span>
      <span class="header-tag">CAPITAL INTELLIGENCE ARCHITECTURE</span>
    </div>
    <div class="header-right">
      <span class="engine-indicator">CORE ENGINE CONVERGENCE</span>
    </div>
  </div>

  <!-- SVG Flowing Capital Ribbons & Core Convergence -->
  <div class="svg-stage">
    <svg
      viewBox="0 0 600 245"
      class="streams-svg"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="Capital Intelligence Multi-Tier Stream Flow"
    >
      <!-- Background Guide Lines (Solid structural lines, no grid) -->
      <g class="guide-lines" aria-hidden="true">
        <line x1="30" y1="12" x2="470" y2="12" class="structural-boundary" />
        <line x1="30" y1="233" x2="470" y2="233" class="structural-boundary" />
        <line x1="160" y1="20" x2="160" y2="225" class="boundary-divider" />
      </g>

      <!-- 6 Flowing Capital Stream Tracks -->
      {#each Object.values(streams) as st}
        {@const isActive = activeKey === st.key}
        <g
          class="stream-track-group"
          class:is-active={isActive}
          class:is-dimmed={activeKey !== 'core' && !isActive}
          role="button"
          tabindex="0"
          aria-label="{st.label}: {st.sublabel}"
          onmouseenter={() => (activeKey = st.key)}
          onfocus={() => (activeKey = st.key)}
          onkeydown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              activeKey = st.key;
            }
          }}
        >
          <!-- Base Wide Track (Solid dark track) -->
          <path d={st.pathD} class="stream-track-bg" />

          <!-- Active / Inflow Solid Ribbon Stroke -->
          <path
            d={st.pathD}
            class="stream-ribbon-path"
            style="stroke: {st.accentColor};"
          />

          <!-- Origin Port Pillar at Start (x=30) -->
          <circle cx="30" cy={st.yStart} r="5" class="origin-port-bg" />
          <circle
            cx="30"
            cy={st.yStart}
            r="3"
            class="origin-port-core"
            style="fill: {st.accentColor};"
          />

          <!-- Intermediate Intake Pin at x=160 -->
          <circle cx="160" cy={st.yStart} r="2.5" class="waypoint-pin" />

          <!-- Dynamic Flow Marker along the Curve (Pulsing marker) -->
          <circle
            cx={st.markerPos.x}
            cy={st.markerPos.y}
            r={isActive ? 4 : 2.5}
            class="flow-marker"
            class:pulse-active={isActive && !prefersReducedMotion}
            style="fill: {st.accentColor};"
          />
        </g>
      {/each}

      <!-- QUANTA COMMAND CORE (Intake & Synthesis Hub) -->
      <g
        class="quanta-core-group"
        class:is-active={activeKey === 'core'}
        role="button"
        tabindex="0"
        aria-label="QUANTA Core Allocation Engine"
        onmouseenter={() => (activeKey = 'core')}
        onfocus={() => (activeKey = 'core')}
        onkeydown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            activeKey = 'core';
          }
        }}
      >
        <!-- Core Intake Port Outer Frame -->
        <rect
          x="475"
          y="72"
          width="112"
          height="100"
          rx="6"
          class="core-frame-outer"
        />

        <!-- Core Interior Panel -->
        <rect
          x="481"
          y="78"
          width="100"
          height="88"
          rx="4"
          class="core-frame-inner"
        />

        <!-- Core Institutional Emblem / Monogram Symbol -->
        <g transform="translate(531, 105)">
          <rect x="-10" y="-10" width="20" height="20" class="core-crest-rect" />
          <line x1="-14" y1="0" x2="14" y2="0" class="core-cross-h" />
          <line x1="0" y1="-14" x2="0" y2="14" class="core-cross-v" />
          <circle cx="0" cy="0" r="4" class="core-crest-dot" />
        </g>

        <!-- Core Typography -->
        <text x="531" y="132" class="core-brand-name" text-anchor="middle">QUANTA</text>
        <text x="531" y="144" class="core-sub-label" text-anchor="middle">SYNTHESIS CORE</text>
        <text x="531" y="157" class="core-status-text" text-anchor="middle">6 STREAMS ACTIVE</text>
      </g>
    </svg>
  </div>

  <!-- Interactive Pillar Selector Pills -->
  <div class="stream-pills-nav" role="tablist" aria-label="Select capital stream to inspect">
    {#each Object.values(streams) as st}
      {@const isSelected = activeKey === st.key}
      <button
        type="button"
        role="tab"
        aria-selected={isSelected}
        class="stream-pill-btn"
        class:is-selected={isSelected}
        onclick={() => (activeKey = st.key)}
      >
        <span class="pill-dot" style="background-color: {st.accentColor};"></span>
        <span class="pill-label">{st.label}</span>
      </button>
    {/each}
    <button
      type="button"
      role="tab"
      aria-selected={activeKey === 'core'}
      class="stream-pill-btn core-pill"
      class:is-selected={activeKey === 'core'}
      onclick={() => (activeKey = 'core')}
    >
      <span class="pill-dot core-dot"></span>
      <span class="pill-label">Core Hub</span>
    </button>
  </div>

  <!-- Detailed Subsystem Inspection Deck -->
  <div class="inspection-deck" aria-live="polite">
    {#if activeStream}
      <div class="inspection-card">
        <div class="card-meta-bar">
          <div class="meta-left">
            <span class="stream-code-tag">{activeStream.code}</span>
            <span class="stream-domain-label">{activeStream.domain}</span>
          </div>
          <div class="meta-right">
            <span class="stream-status-pill" style="border-color: {activeStream.accentColor}; color: {activeStream.accentColor};">
              CONNECTED
            </span>
          </div>
        </div>

        <div class="card-content-grid">
          <div class="info-block">
            <span class="block-label">OPERATIONAL MECHANISM</span>
            <p class="block-text">{activeStream.mechanism}</p>
          </div>
          <div class="info-block">
            <span class="block-label">GOVERNANCE &amp; CONTROL RULE</span>
            <p class="block-text">{activeStream.operationalRule}</p>
          </div>
        </div>
      </div>
    {:else}
      <!-- QUANTA Core Overview -->
      <div class="inspection-card core-inspection">
        <div class="card-meta-bar">
          <div class="meta-left">
            <span class="stream-code-tag core-code">CORE ENGINE</span>
            <span class="stream-domain-label">Autonomous Balance-Sheet Synthesis</span>
          </div>
          <div class="meta-right">
            <span class="stream-status-pill core-status">OPERATIONAL</span>
          </div>
        </div>

        <div class="card-content-grid">
          <div class="info-block">
            <span class="block-label">SYNTHESIS ENGINE</span>
            <p class="block-text">
              Unifies assets, liabilities, and liquidity reserves into one deterministic mathematical state.
            </p>
          </div>
          <div class="info-block">
            <span class="block-label">DISCIPLINED EXECUTION</span>
            <p class="block-text">
              Bayesian stress conditioning and heuristic Pareto solvers eliminate manual balance sheet allocation latency.
            </p>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  /* Root Container */
  .capital-streams-root {
    display: flex;
    flex-direction: column;
    width: 100%;
    background-color: #0b0f19;
    border: 1px solid #1e293b;
    border-radius: 8px;
    padding: 1.15rem 1.25rem;
    box-sizing: border-box;
    gap: 0.85rem;
  }

  /* Header */
  .streams-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid #1e293b;
    padding-bottom: 0.65rem;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .status-marker {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: #10b981;
    display: inline-block;
  }

  .header-tag {
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #cbd5e1;
    text-transform: uppercase;
  }

  .engine-indicator {
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    color: #64748b;
  }

  /* SVG Canvas Stage */
  .svg-stage {
    width: 100%;
    display: flex;
    justify-content: center;
    align-items: center;
  }

  .streams-svg {
    width: 100%;
    max-height: 200px;
    display: block;
    overflow: visible;
  }

  /* Guide Lines */
  .structural-boundary {
    stroke: #1e293b;
    stroke-width: 1;
    stroke-dasharray: 4, 4;
  }

  .boundary-divider {
    stroke: #172033;
    stroke-width: 1;
  }

  /* Stream Tracks */
  .stream-track-group {
    cursor: pointer;
    transition: opacity 0.2s ease;
    outline: none;
  }

  .stream-track-group.is-dimmed {
    opacity: 0.22;
  }

  .stream-track-group.is-active {
    opacity: 1;
  }

  .stream-track-bg {
    stroke: #131d2e;
    stroke-width: 7;
    fill: none;
    stroke-linecap: round;
  }

  .stream-ribbon-path {
    stroke-width: 2.25;
    fill: none;
    stroke-linecap: round;
    transition: stroke-width 0.2s ease;
  }

  .stream-track-group.is-active .stream-ribbon-path {
    stroke-width: 3.5;
  }

  .origin-port-bg {
    fill: #0b0f19;
    stroke: #334155;
    stroke-width: 1.5;
  }

  .origin-port-core {
    transition: transform 0.2s ease;
  }

  .waypoint-pin {
    fill: #334155;
  }

  .flow-marker {
    transition: r 0.2s ease;
  }

  @keyframes pulseMarker {
    0%, 100% {
      opacity: 0.6;
      transform: scale(1);
    }
    50% {
      opacity: 1;
      transform: scale(1.4);
    }
  }

  .pulse-active {
    animation: pulseMarker 1.8s infinite ease-in-out;
    transform-origin: center;
  }

  /* QUANTA Command Core */
  .quanta-core-group {
    cursor: pointer;
    outline: none;
    transition: transform 0.2s ease;
  }

  .core-frame-outer {
    fill: #070b14;
    stroke: #334155;
    stroke-width: 1.5;
    transition: stroke 0.2s ease;
  }

  .quanta-core-group.is-active .core-frame-outer,
  .quanta-core-group:hover .core-frame-outer {
    stroke: #d4af37;
  }

  .core-frame-inner {
    fill: #0f172a;
    stroke: #1e293b;
    stroke-width: 1;
  }

  .core-crest-rect {
    fill: #070b14;
    stroke: #d4af37;
    stroke-width: 1.25;
  }

  .core-cross-h,
  .core-cross-v {
    stroke: #d4af37;
    stroke-width: 1;
  }

  .core-crest-dot {
    fill: #d4af37;
  }

  .core-brand-name {
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 0.12em;
    fill: #ffffff;
  }

  .core-sub-label {
    font-size: 7.5px;
    font-weight: 600;
    letter-spacing: 0.08em;
    fill: #94a3b8;
  }

  .core-status-text {
    font-size: 6.5px;
    font-weight: 700;
    letter-spacing: 0.06em;
    fill: #10b981;
  }

  /* Pills Navigation Row */
  .stream-pills-nav {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    align-items: center;
  }

  .stream-pill-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background-color: #070b14;
    border: 1px solid #1e293b;
    border-radius: 4px;
    padding: 0.25rem 0.55rem;
    font-size: 0.6875rem;
    font-weight: 500;
    color: #94a3b8;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .stream-pill-btn:hover {
    color: #ffffff;
    border-color: #334155;
    background-color: #0f172a;
  }

  .stream-pill-btn.is-selected {
    color: #ffffff;
    border-color: #d4af37;
    background-color: #172033;
  }

  .pill-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .core-dot {
    background-color: #d4af37;
  }

  /* Inspection Deck */
  .inspection-deck {
    width: 100%;
  }

  .inspection-card {
    background-color: #070b14;
    border: 1px solid #1e293b;
    border-radius: 6px;
    padding: 0.75rem 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .core-inspection {
    border-color: #334155;
  }

  .card-meta-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid #172033;
    padding-bottom: 0.4rem;
  }

  .meta-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .stream-code-tag {
    font-size: 0.625rem;
    font-weight: 700;
    color: #d4af37;
    background-color: #172033;
    padding: 0.15rem 0.35rem;
    border-radius: 3px;
    letter-spacing: 0.05em;
  }

  .core-code {
    color: #10b981;
  }

  .stream-domain-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #f1f5f9;
  }

  .stream-status-pill {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    padding: 0.15rem 0.4rem;
    border: 1px solid #10b981;
    border-radius: 3px;
    text-transform: uppercase;
  }

  .core-status {
    border-color: #10b981;
    color: #10b981;
  }

  .card-content-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
  }

  .info-block {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .block-label {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #64748b;
    text-transform: uppercase;
  }

  .block-text {
    font-size: 0.71875rem;
    line-height: 1.35;
    color: #cbd5e1;
    margin: 0;
  }

  @media (max-width: 640px) {
    .card-content-grid {
      grid-template-columns: 1fr;
      gap: 0.5rem;
    }

    .streams-svg {
      max-height: 160px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .pulse-active {
      animation: none;
    }

    .stream-track-group,
    .stream-ribbon-path,
    .stream-pill-btn,
    .core-frame-outer {
      transition: none;
    }
  }
</style>
