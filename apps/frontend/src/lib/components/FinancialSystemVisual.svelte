<script lang="ts">
  // QUANTA Financial System Visualization
  // Custom-designed institutional balance-sheet and capital allocation visual.
  // Built purely with HTML, CSS, and SVG.
  // Zero photographs, zero gradients, zero sci-fi elements, zero emojis.
</script>

<div class="financial-visual-root" aria-label="QUANTA Balance Sheet & Capital Allocation Visual">
  <!-- Subtle Institutional Architectural Ledger Grid (Pure SVG, no gradients) -->
  <div class="grid-backdrop" aria-hidden="true">
    <svg class="grid-svg" width="100%" height="100%" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <pattern id="ledger-grid-pattern" width="40" height="40" patternUnits="userSpaceOnUse">
          <path d="M 40 0 L 0 0 0 40" fill="none" stroke="currentColor" stroke-width="0.75" />
        </pattern>
      </defs>
      <rect width="100%" height="100%" fill="url(#ledger-grid-pattern)" />
    </svg>
  </div>

  <div class="visual-content-container">
    <!-- Header Badge -->
    <div class="model-meta-header">
      <div class="meta-left">
        <span class="meta-indicator-dot"></span>
        <span class="meta-title">BALANCE SHEET ARCHITECTURE</span>
      </div>
      <span class="meta-tag">BENCHMARK MODEL // Q-8094</span>
    </div>

    <!-- PRIMARY PANEL: Balance Sheet Structure -->
    <div class="financial-card balance-card">
      <div class="card-inner-header">
        <div class="section-title-group">
          <span class="section-eyebrow">CONSOLIDATED TREASURY STRUCTURE</span>
          <h2 class="section-heading">Assets vs Liabilities Overview</h2>
        </div>
        <div class="equation-pill">
          <span class="pill-kpi-label">NET SURPLUS</span>
          <span class="pill-kpi-val">$75.80M</span>
        </div>
      </div>

      <!-- Assets Segment -->
      <div class="ledger-segment">
        <div class="segment-meta-row">
          <span class="segment-label">TOTAL ASSETS</span>
          <span class="segment-total">$128,450,000</span>
        </div>
        <div class="ledger-rows">
          <div class="ledger-item">
            <div class="item-info">
              <span class="item-name">Cash & Cash Equivalents</span>
              <span class="item-nums">$41.10M <span class="num-pct">32.0%</span></span>
            </div>
            <div class="bar-track">
              <div class="bar-fill bar-cash" style="width: 32%;"></div>
            </div>
          </div>

          <div class="ledger-item">
            <div class="item-info">
              <span class="item-name">Investment Portfolio</span>
              <span class="item-nums">$59.10M <span class="num-pct">46.0%</span></span>
            </div>
            <div class="bar-track">
              <div class="bar-fill bar-portfolio" style="width: 46%;"></div>
            </div>
          </div>

          <div class="ledger-item">
            <div class="item-info">
              <span class="item-name">Operational Receivables & Collateral</span>
              <span class="item-nums">$28.25M <span class="num-pct">22.0%</span></span>
            </div>
            <div class="bar-track">
              <div class="bar-fill bar-receivables" style="width: 22%;"></div>
            </div>
          </div>
        </div>
      </div>

      <div class="segment-divider"></div>

      <!-- Liabilities Segment -->
      <div class="ledger-segment">
        <div class="segment-meta-row">
          <span class="segment-label segment-label-liability">TOTAL LIABILITIES</span>
          <span class="segment-total segment-total-liability">$52,650,000</span>
        </div>
        <div class="ledger-rows">
          <div class="ledger-item">
            <div class="item-info">
              <span class="item-name">Senior Credit Facilities</span>
              <span class="item-nums">$34.20M <span class="num-pct">65.0%</span></span>
            </div>
            <div class="bar-track">
              <div class="bar-fill bar-debt" style="width: 65%;"></div>
            </div>
          </div>

          <div class="ledger-item">
            <div class="item-info">
              <span class="item-name">Working Capital & Payables</span>
              <span class="item-nums">$18.45M <span class="num-pct">35.0%</span></span>
            </div>
            <div class="bar-track">
              <div class="bar-fill bar-payables" style="width: 35%;"></div>
            </div>
          </div>
        </div>
      </div>

      <!-- Ratio Summary Footer -->
      <div class="balance-footer-strip">
        <div class="strip-item">
          <span class="strip-label">COVERAGE RATIO</span>
          <span class="strip-val">2.44x</span>
        </div>
        <div class="strip-divider"></div>
        <div class="strip-item">
          <span class="strip-label">TIER 1 ADEQUACY</span>
          <span class="strip-val status-positive">Optimal</span>
        </div>
        <div class="strip-divider"></div>
        <div class="strip-item">
          <span class="strip-label">LIQUIDITY RUNWAY</span>
          <span class="strip-val">18.4 Mo</span>
        </div>
      </div>
    </div>

    <!-- SECONDARY PANEL: Capital Allocation & Liquidity Distribution -->
    <div class="financial-card allocation-card">
      <div class="card-inner-header">
        <div class="section-title-group">
          <span class="section-eyebrow">PORTFOLIO MANDATE</span>
          <h3 class="section-subheading">Capital Allocation Breakdown</h3>
        </div>
        <span class="allocation-badge">OPTIMIZED</span>
      </div>

      <!-- Segmented Stacked Bar (Solid blocks, thin 1px separators) -->
      <div class="stacked-bar-container" aria-label="Capital Allocation: Cash 32%, Bonds 28%, Equities 24%, Other 16%">
        <div class="stacked-segment seg-cash" style="width: 32%;"></div>
        <div class="stacked-separator"></div>
        <div class="stacked-segment seg-bonds" style="width: 28%;"></div>
        <div class="stacked-separator"></div>
        <div class="stacked-segment seg-equities" style="width: 24%;"></div>
        <div class="stacked-separator"></div>
        <div class="stacked-segment seg-other" style="width: 16%;"></div>
      </div>

      <!-- 4-Column Allocation Matrix -->
      <div class="allocation-grid">
        <div class="matrix-cell">
          <div class="matrix-header">
            <span class="legend-swatch swatch-cash"></span>
            <span class="matrix-title">CASH</span>
          </div>
          <div class="matrix-pct">32%</div>
          <div class="matrix-sub">Yield 4.85%</div>
        </div>

        <div class="matrix-cell">
          <div class="matrix-header">
            <span class="legend-swatch swatch-bonds"></span>
            <span class="matrix-title">BONDS</span>
          </div>
          <div class="matrix-pct">28%</div>
          <div class="matrix-sub">Dur 2.4y</div>
        </div>

        <div class="matrix-cell">
          <div class="matrix-header">
            <span class="legend-swatch swatch-equities"></span>
            <span class="matrix-title">EQUITIES</span>
          </div>
          <div class="matrix-pct">24%</div>
          <div class="matrix-sub">Beta 0.72</div>
        </div>

        <div class="matrix-cell">
          <div class="matrix-header">
            <span class="legend-swatch swatch-other"></span>
            <span class="matrix-title">OTHER</span>
          </div>
          <div class="matrix-pct">16%</div>
          <div class="matrix-sub">VaR 1.15%</div>
        </div>
      </div>

      <!-- Small Institutional Stepped Liquidity Sparkline (Pure SVG, crisp solid stroke) -->
      <div class="liquidity-vector-strip">
        <div class="vector-meta">
          <span class="vector-label">PROJECTED LIQUIDITY FLOOR</span>
          <span class="vector-value">+$16.1M above threshold</span>
        </div>
        <div class="sparkline-wrapper">
          <svg class="sparkline-svg" viewBox="0 0 320 32" fill="none" preserveAspectRatio="none">
            <!-- Dashed Minimum Reserve Threshold -->
            <line x1="0" y1="20" x2="320" y2="20" stroke="var(--vis-border, #334155)" stroke-width="1" stroke-dasharray="3 3"/>
            <!-- Solid Stepped Liquidity Curve -->
            <path d="M 0 24 L 40 22 L 80 18 L 120 19 L 160 14 L 200 15 L 240 10 L 280 8 L 320 6" stroke="var(--vis-gold, #D4AF37)" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round"/>
            <!-- End Node -->
            <circle cx="320" cy="6" r="3" fill="var(--vis-gold, #D4AF37)" />
          </svg>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  .financial-visual-root {
    position: relative;
    width: 100%;
    border-radius: 8px;
    background-color: var(--vis-surface-bg, #0B1017);
    border: 1px solid var(--vis-border, #1E293B);
    overflow: hidden;
    box-sizing: border-box;
  }

  /* Grid Backdrop: Crisp solid lines */
  .grid-backdrop {
    position: absolute;
    inset: 0;
    pointer-events: none;
    opacity: 0.35;
    color: #1E293B;
  }

  :global([data-theme="light"]) .grid-backdrop {
    opacity: 0.45;
    color: #E2E8F0;
  }

  .visual-content-container {
    position: relative;
    z-index: 2;
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.15rem;
  }

  /* Header Badge */
  .model-meta-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--vis-border-subtle, #172233);
  }

  .meta-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .meta-indicator-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: #D4AF37;
  }

  :global([data-theme="light"]) .meta-indicator-dot {
    background-color: #B8860B;
  }

  .meta-title {
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.1em;
    color: var(--vis-text-muted, #94A3B8);
    font-family: var(--font-mono, monospace);
  }

  .meta-tag {
    font-size: 0.65rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    color: var(--vis-text-subtle, #64748B);
    font-family: var(--font-mono, monospace);
  }

  /* Financial Cards */
  .financial-card {
    background-color: var(--vis-card-bg, #0F1724);
    border: 1px solid var(--vis-card-border, #1E2B3E);
    border-radius: 6px;
    padding: 1.25rem;
    box-sizing: border-box;
  }

  :global([data-theme="light"]) .financial-card {
    background-color: #FFFFFF;
    border: 1px solid #E2E8F0;
  }

  .card-inner-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    margin-bottom: 1rem;
    gap: 0.75rem;
  }

  .section-eyebrow {
    display: block;
    font-size: 0.625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #D4AF37;
    margin-bottom: 0.2rem;
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .section-eyebrow {
    color: #B8860B;
  }

  .section-heading {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--vis-text-primary, #F8FAFC);
    letter-spacing: -0.01em;
    margin: 0;
  }

  :global([data-theme="light"]) .section-heading {
    color: #0F172A;
  }

  .section-subheading {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--vis-text-primary, #F8FAFC);
    margin: 0;
  }

  :global([data-theme="light"]) .section-subheading {
    color: #0F172A;
  }

  .equation-pill {
    display: inline-flex;
    flex-direction: column;
    align-items: flex-end;
    background-color: var(--vis-badge-bg, #141D2C);
    border: 1px solid var(--vis-badge-border, #24334A);
    padding: 0.35rem 0.65rem;
    border-radius: 4px;
  }

  :global([data-theme="light"]) .equation-pill {
    background-color: #F1F5F9;
    border: 1px solid #CBD5E1;
  }

  .pill-kpi-label {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: #94A3B8;
    font-family: var(--font-mono, monospace);
  }

  .pill-kpi-val {
    font-size: 0.8125rem;
    font-weight: 700;
    color: #D4AF37;
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .pill-kpi-val {
    color: #B8860B;
  }

  /* Ledger Structure */
  .ledger-segment {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }

  .segment-meta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .segment-label {
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: #38BDF8;
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .segment-label {
    color: #0284C7;
  }

  .segment-label-liability {
    color: #94A3B8;
  }

  :global([data-theme="light"]) .segment-label-liability {
    color: #64748B;
  }

  .segment-total {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--vis-text-primary, #F8FAFC);
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .segment-total {
    color: #0F172A;
  }

  .segment-total-liability {
    color: #CBD5E1;
  }

  :global([data-theme="light"]) .segment-total-liability {
    color: #475569;
  }

  .ledger-rows {
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .ledger-item {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .item-info {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.75rem;
  }

  .item-name {
    color: var(--vis-text-muted, #94A3B8);
  }

  :global([data-theme="light"]) .item-name {
    color: #475569;
  }

  .item-nums {
    font-weight: 600;
    color: var(--vis-text-primary, #F8FAFC);
    font-family: var(--font-mono, monospace);
    font-size: 0.75rem;
  }

  :global([data-theme="light"]) .item-nums {
    color: #0F172A;
  }

  .num-pct {
    color: var(--vis-text-subtle, #64748B);
    font-size: 0.6875rem;
    margin-left: 0.35rem;
  }

  /* Bar Tracks & Solid Fills (No gradients) */
  .bar-track {
    width: 100%;
    height: 4px;
    background-color: var(--vis-track-bg, #1A2333);
    border-radius: 2px;
    overflow: hidden;
  }

  :global([data-theme="light"]) .bar-track {
    background-color: #E2E8F0;
  }

  .bar-fill {
    height: 100%;
  }

  .bar-cash {
    background-color: #D4AF37;
  }

  :global([data-theme="light"]) .bar-cash {
    background-color: #B8860B;
  }

  .bar-portfolio {
    background-color: #38BDF8;
  }

  :global([data-theme="light"]) .bar-portfolio {
    background-color: #0284C7;
  }

  .bar-receivables {
    background-color: #64748B;
  }

  .bar-debt {
    background-color: #475569;
  }

  :global([data-theme="light"]) .bar-debt {
    background-color: #64748B;
  }

  .bar-payables {
    background-color: #94A3B8;
  }

  :global([data-theme="light"]) .bar-payables {
    background-color: #94A3B8;
  }

  .segment-divider {
    height: 1px;
    background-color: var(--vis-border-subtle, #172233);
    margin: 0.75rem 0;
  }

  :global([data-theme="light"]) .segment-divider {
    background-color: #E2E8F0;
  }

  /* Ratio Strip Footer */
  .balance-footer-strip {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 1rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--vis-border-subtle, #172233);
  }

  :global([data-theme="light"]) .balance-footer-strip {
    border-top: 1px solid #E2E8F0;
  }

  .strip-item {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .strip-label {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--vis-text-subtle, #64748B);
    font-family: var(--font-mono, monospace);
  }

  .strip-val {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--vis-text-primary, #F8FAFC);
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .strip-val {
    color: #0F172A;
  }

  .status-positive {
    color: #10B981;
  }

  :global([data-theme="light"]) .status-positive {
    color: #059669;
  }

  .strip-divider {
    width: 1px;
    height: 20px;
    background-color: var(--vis-border-subtle, #172233);
  }

  :global([data-theme="light"]) .strip-divider {
    background-color: #E2E8F0;
  }

  /* Allocation Panel */
  .allocation-badge {
    font-size: 0.625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    padding: 0.2rem 0.5rem;
    background-color: #10B981;
    color: #080D14;
    border-radius: 3px;
    font-family: var(--font-mono, monospace);
  }

  /* Stacked Bar */
  .stacked-bar-container {
    display: flex;
    width: 100%;
    height: 8px;
    border-radius: 4px;
    overflow: hidden;
    background-color: var(--vis-track-bg, #1A2333);
    margin-bottom: 0.85rem;
  }

  .stacked-segment {
    height: 100%;
  }

  .stacked-separator {
    width: 1px;
    height: 100%;
    background-color: var(--vis-card-bg, #0F1724);
  }

  :global([data-theme="light"]) .stacked-separator {
    background-color: #FFFFFF;
  }

  .seg-cash {
    background-color: #D4AF37;
  }

  :global([data-theme="light"]) .seg-cash {
    background-color: #B8860B;
  }

  .seg-bonds {
    background-color: #2563EB;
  }

  :global([data-theme="light"]) .seg-bonds {
    background-color: #1D4ED8;
  }

  .seg-equities {
    background-color: #0EA5E9;
  }

  :global([data-theme="light"]) .seg-equities {
    background-color: #0284C7;
  }

  .seg-other {
    background-color: #64748B;
  }

  /* 4-Column Allocation Matrix */
  .allocation-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.5rem;
    margin-bottom: 0.85rem;
  }

  .matrix-cell {
    background-color: var(--vis-cell-bg, #141D2C);
    border: 1px solid var(--vis-badge-border, #24334A);
    border-radius: 4px;
    padding: 0.5rem;
    box-sizing: border-box;
  }

  :global([data-theme="light"]) .matrix-cell {
    background-color: #F8FAFC;
    border: 1px solid #E2E8F0;
  }

  .matrix-header {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    margin-bottom: 0.25rem;
  }

  .legend-swatch {
    width: 6px;
    height: 6px;
    border-radius: 1px;
  }

  .swatch-cash {
    background-color: #D4AF37;
  }

  :global([data-theme="light"]) .swatch-cash {
    background-color: #B8860B;
  }

  .swatch-bonds {
    background-color: #2563EB;
  }

  .swatch-equities {
    background-color: #0EA5E9;
  }

  .swatch-other {
    background-color: #64748B;
  }

  .matrix-title {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--vis-text-subtle, #94A3B8);
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .matrix-title {
    color: #475569;
  }

  .matrix-pct {
    font-size: 0.875rem;
    font-weight: 700;
    color: var(--vis-text-primary, #F8FAFC);
    font-family: var(--font-mono, monospace);
    line-height: 1.1;
  }

  :global([data-theme="light"]) .matrix-pct {
    color: #0F172A;
  }

  .matrix-sub {
    font-size: 0.625rem;
    color: var(--vis-text-subtle, #64748B);
    margin-top: 0.15rem;
    font-family: var(--font-mono, monospace);
  }

  /* Liquidity Vector Strip */
  .liquidity-vector-strip {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    padding-top: 0.65rem;
    border-top: 1px solid var(--vis-border-subtle, #172233);
  }

  :global([data-theme="light"]) .liquidity-vector-strip {
    border-top: 1px solid #E2E8F0;
  }

  .vector-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .vector-label {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--vis-text-subtle, #64748B);
    font-family: var(--font-mono, monospace);
  }

  .vector-value {
    font-size: 0.625rem;
    font-weight: 600;
    color: #10B981;
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="light"]) .vector-value {
    color: #059669;
  }

  .sparkline-wrapper {
    width: 100%;
    height: 24px;
  }

  .sparkline-svg {
    width: 100%;
    height: 100%;
  }

  /* Responsive Adjustments */
  @media (max-width: 768px) {
    .visual-content-container {
      padding: 0.85rem;
      gap: 0.85rem;
    }

    .financial-card {
      padding: 0.85rem;
    }

    .card-inner-header {
      flex-direction: column;
      align-items: flex-start;
      gap: 0.5rem;
    }

    .equation-pill {
      align-self: flex-start;
      align-items: flex-start;
    }

    .section-heading {
      font-size: 0.875rem;
    }

    .section-subheading {
      font-size: 0.8125rem;
    }

    .item-info {
      font-size: 0.6875rem;
    }

    .item-nums {
      font-size: 0.6875rem;
    }

    .balance-footer-strip {
      flex-wrap: wrap;
      gap: 0.65rem;
    }

    .strip-divider {
      display: none;
    }

    .allocation-grid {
      grid-template-columns: repeat(2, 1fr);
      gap: 0.4rem;
    }

    .matrix-pct {
      font-size: 0.8125rem;
    }

    .matrix-sub {
      font-size: 0.5625rem;
    }
  }
</style>
