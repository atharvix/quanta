<script lang="ts">
  let activeView = $state<'ledger' | 'solvers' | 'stress'>('ledger');

  const ledgerRows = [
    { instrument: 'US Treasury Bills (3M)', category: 'sovereign', weight: '42.0%', amount: '$50.4M', yieldPct: '+4.85%', risk: '0.04' },
    { instrument: 'Investment-Grade Credit', category: 'fixed income', weight: '34.0%', amount: '$40.8M', yieldPct: '+6.15%', risk: '0.18' },
    { instrument: 'Commercial Paper Pool', category: 'money market', weight: '24.0%', amount: '$28.8M', yieldPct: '+5.10%', risk: '0.09' }
  ];

  const solverRows = [
    { solver: 'Exact Enumeration', status: 'Optimal', objective: '0.0842', runtime: '1.14 ms', feasible: 'Yes' },
    { solver: 'SciPy SLSQP', status: 'Converged', objective: '0.0859', runtime: '3.82 ms', feasible: 'Yes' },
    { solver: 'QAOA Statevector (p=2)', status: 'Simulated', objective: '0.0817', runtime: '48.60 ms', feasible: 'Yes' }
  ];

  const stressRows = [
    { regime: 'Base Policy', pRecession: '14%', liquidity: '28.4%', solvency: 'Solvent' },
    { regime: 'Rate Hike (+250 bps)', pRecession: '38%', liquidity: '24.1%', solvency: 'Solvent' },
    { regime: 'Liquidity Crunch', pRecession: '64%', liquidity: '17.8%', solvency: 'Breach' }
  ];
</script>

<div class="ledger-frame" role="region" aria-label="Sample QUANTA Workspace Sheet">
  <div class="frame-topbar">
    <div class="frame-entity">
      <span class="entity-name">Meridian Global Treasury</span>
      <span class="entity-sep">/</span>
      <span class="entity-ccy font-mono">USD &middot; $120.0M</span>
    </div>
    <div class="frame-tabs" role="tablist" aria-label="Preview views">
      <button
        type="button"
        role="tab"
        aria-selected={activeView === 'ledger'}
        class="frame-tab"
        class:active={activeView === 'ledger'}
        onclick={() => (activeView = 'ledger')}
      >
        Allocations
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={activeView === 'solvers'}
        class="frame-tab"
        class:active={activeView === 'solvers'}
        onclick={() => (activeView = 'solvers')}
      >
        Solvers
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={activeView === 'stress'}
        class="frame-tab"
        class:active={activeView === 'stress'}
        onclick={() => (activeView = 'stress')}
      >
        Stress test
      </button>
    </div>
  </div>

  {#if activeView === 'ledger'}
    <table class="preview-table">
      <thead>
        <tr>
          <th>Instrument</th>
          <th>Class</th>
          <th class="num">Weight</th>
          <th class="num">Deployed</th>
          <th class="num">Exp. Yield</th>
          <th class="num">Risk</th>
        </tr>
      </thead>
      <tbody>
        {#each ledgerRows as row}
          <tr>
            <td class="cell-strong">{row.instrument}</td>
            <td class="cell-muted">{row.category}</td>
            <td class="num font-mono">{row.weight}</td>
            <td class="num font-mono">{row.amount}</td>
            <td class="num font-mono pos">{row.yieldPct}</td>
            <td class="num font-mono">{row.risk}</td>
          </tr>
        {/each}
      </tbody>
    </table>
    <div class="frame-foot">
      <span>Min liquidity floor: 20.0%</span>
      <span class="font-mono">Debt coverage: 4.80x</span>
    </div>
  {:else if activeView === 'solvers'}
    <table class="preview-table">
      <thead>
        <tr>
          <th>Method</th>
          <th>Status</th>
          <th class="num">Objective</th>
          <th class="num">Runtime</th>
          <th class="num">Feasible</th>
        </tr>
      </thead>
      <tbody>
        {#each solverRows as row}
          <tr>
            <td class="cell-strong">{row.solver}</td>
            <td class="cell-muted">{row.status}</td>
            <td class="num font-mono">{row.objective}</td>
            <td class="num font-mono">{row.runtime}</td>
            <td class="num pos">{row.feasible}</td>
          </tr>
        {/each}
      </tbody>
    </table>
    <div class="frame-foot">
      <span>CPU QUBO Statevector · 1,024 shots</span>
      <span class="font-mono">Approx. ratio: 0.9703</span>
    </div>
  {:else}
    <table class="preview-table">
      <thead>
        <tr>
          <th>Macro Regime</th>
          <th class="num">P(Recession)</th>
          <th class="num">Stressed Liquidity</th>
          <th class="num">Policy Check</th>
        </tr>
      </thead>
      <tbody>
        {#each stressRows as row}
          <tr>
            <td class="cell-strong">{row.regime}</td>
            <td class="num font-mono">{row.pRecession}</td>
            <td class="num font-mono">{row.liquidity}</td>
            <td class="num" class:pos={row.solvency === 'Solvent'} class:neg={row.solvency === 'Breach'}>
              {row.solvency}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <div class="frame-foot">
      <span>Bayesian Network + Minimax Tree</span>
      <span class="font-mono">95% VaR: -4.2%</span>
    </div>
  {/if}
</div>

<style>
  .ledger-frame {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    overflow: hidden;
  }

  .frame-topbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    padding: 0 1rem;
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-bg-secondary);
    flex-wrap: wrap;
  }

  .frame-entity {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.7rem 0;
    font-size: 0.8125rem;
  }

  .entity-name {
    font-weight: 500;
    color: var(--color-text);
  }

  .entity-sep {
    color: var(--color-border-strong);
  }

  .entity-ccy {
    color: var(--color-text-muted);
  }

  .frame-tabs {
    display: flex;
    gap: 1rem;
  }

  .frame-tab {
    background: transparent;
    border: none;
    border-bottom: 2px solid transparent;
    padding: 0.7rem 0.1rem;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-text-muted);
    cursor: pointer;
    transition: color 140ms var(--ease), border-color 140ms var(--ease);
  }

  .frame-tab:hover {
    color: var(--color-text);
  }

  .frame-tab.active {
    color: var(--color-text);
    border-bottom-color: var(--color-primary);
  }

  .preview-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
    text-align: left;
  }

  .preview-table th {
    padding: 0.55rem 1rem;
    font-weight: 500;
    color: var(--color-text-muted);
    border-bottom: 1px solid var(--color-border);
    white-space: nowrap;
  }

  .preview-table td {
    padding: 0.65rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
    font-variant-numeric: tabular-nums;
  }

  .preview-table tbody tr:last-child td {
    border-bottom: none;
  }

  .num {
    text-align: right;
  }

  .cell-strong {
    font-weight: 500;
  }

  .cell-muted {
    color: var(--color-text-secondary);
  }

  .pos {
    color: var(--color-success);
    font-weight: 500;
  }

  .neg {
    color: var(--color-danger);
    font-weight: 500;
  }

  .frame-foot {
    display: flex;
    justify-content: space-between;
    padding: 0.6rem 1rem;
    border-top: 1px solid var(--color-border);
    background-color: var(--color-bg-secondary);
    font-size: 0.75rem;
    color: var(--color-text-muted);
    gap: 1rem;
    flex-wrap: wrap;
  }
</style>
