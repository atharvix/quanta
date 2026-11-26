export interface Treasury {
  id: string;
  name: string;
  base_currency: string;
  total_capital: number;
  risk_tolerance: number;
  min_liquidity_ratio: number;
  created_at: string;
  updated_at: string;
}

export interface CreateTreasuryInput {
  name: string;
  base_currency: string;
  total_capital: number;
  risk_tolerance: number;
  min_liquidity_ratio: number;
}

export interface UpdateTreasuryInput {
  name?: string;
  base_currency?: string;
  total_capital?: number;
  risk_tolerance?: number;
  min_liquidity_ratio?: number;
}

export interface TreasuriesListResponse {
  data: Treasury[];
  limit: number;
  offset: number;
}

export interface StrategyExecutionLog {
  id: string;
  treasury_id: string;
  algorithm_used: string;
  objective_value: number;
  risk_score: number;
  expected_return: number;
  liquidity_ratio: number;
  allocations_json: Record<string, number> | any;
  constraints_audit_json?: any;
  execution_time_ms: number;
  created_at: string;
}

export interface StrategiesListResponse {
  data: StrategyExecutionLog[];
  limit: number;
  offset: number;
}

export interface Asset {
  id: string;
  name: string;
  asset_type: string;
  cost: number;
  expected_return: number;
  risk: number;
  liquidity: number;
  volatility?: number;
  metadata?: Record<string, any>;
}

export interface TreasuryAssetsResponse {
  data: Asset[];
}

export interface Liability {
  id: string;
  name: string;
  principal: number;
  interest_rate: number;
  maturity_months?: number;
  priority: number;
  is_mandatory_reserve: boolean;
}

export interface TreasuryLiabilitiesResponse {
  data: Liability[];
}

export interface Constraint {
  id: string;
  constraint_type: string;
  description?: string;
  parameters: Record<string, any>;
}

export interface OrchestrateAllocationInput {
  treasury_id: string;
  algorithm: string;
  step_size?: number;
  assets: Asset[];
  liabilities: Liability[];
  constraints: Constraint[];
  objective?: string;
  market_state?: string;
  include_comparison: boolean;
}

export interface ConstraintAuditItem {
  constraint_id?: string;
  constraint_type: string;
  satisfied: boolean;
  actual: number;
  required: number;
  severity?: string;
  message?: string;
  penalty?: number;
}

export interface StrategyExecutionResponse extends StrategyExecutionLog {
  explanation: string;
}

export interface OptimizationStatusResponse {
  status: string;
  reason: string;
  best_classical_method?: string;
  qaoa_approximation_ratio?: number | null;
  simulation_note?: string;
  problem_summary?: Record<string, any>;
  summary_table?: OptimizationSummaryRow[];
}

// Optimization Engine (Classical vs. Quantum QAOA Lab) — matches shared/schemas/optimization_*.json
export type OptimizationMethod = 'exact' | 'greedy' | 'sa' | 'slsqp' | 'milp' | 'qaoa' | string;

export interface OptimizationAsset extends Asset {
  min_investment?: number;
  max_investment?: number;
  group?: string;
}

export interface OptimizationLiability extends Partial<Liability> {
  amount?: number;
  interest_rate: number;
  maturity?: number;
}

export interface OptimizationConstraint extends Constraint {
  severity?: string;
  penalty_weight?: number;
}

export interface OptimizationProblemPayload {
  capital: number;
  assets: OptimizationAsset[];
  liabilities: OptimizationLiability[];
  constraints: OptimizationConstraint[];
  risk_tolerance: number;
  minimum_liquidity: number;
  objective: string;
  step_size?: number;
  max_exposure_pct?: number;
  group_caps?: Record<string, number>;
  covariance_matrix?: number[][];
  candidate_subset?: string[];
  market_state?: string;
}

export interface OptimizationOptions {
  p?: number;
  shots?: number;
  seed?: number;
  max_iterations?: number;
  use_slack?: boolean;
  [key: string]: any;
}

export interface OptimizationRequestPayload {
  problem: OptimizationProblemPayload;
  method?: OptimizationMethod;
  options?: OptimizationOptions;
  candidate_subset?: string[];
}

export interface OptimizationCompareRequestPayload {
  problem: OptimizationProblemPayload;
  options?: OptimizationOptions;
  candidate_subset?: string[];
}

export interface OptimizationResult {
  method: string;
  status: 'optimal' | 'feasible' | 'infeasible' | 'error' | string;
  allocation: Record<string, number>;
  objective_value: number;
  portfolio_return: number;
  portfolio_risk: number;
  liquidity: number;
  capital_used: number;
  feasible: boolean;
  constraint_violations: string[];
  runtime_ms: number;
  metadata: Record<string, any>;
}

export interface OptimizationSummaryRow {
  method: string;
  status: string;
  objective_value: number;
  portfolio_return: number;
  portfolio_risk: number;
  liquidity: number;
  capital_used: number;
  feasible: boolean;
  violations_count?: number;
  runtime_ms: number;
  [key: string]: any;
}

export interface OptimizationCompareResponse {
  problem_summary: Record<string, any>;
  results: Record<string, OptimizationResult>;
  summary_table: OptimizationSummaryRow[];
  best_classical_method: string;
  qaoa_approximation_ratio?: number | null;
  simulation_note: string;
}

export interface SearchResult {
  algorithm: string;
  success: boolean;
  objective_value: number;
  expected_return: number;
  risk_score: number;
  liquidity_ratio: number;
  states_explored: number;
  execution_time_ms: number;
  depth?: number;
  final_state?: {
    allocations?: Record<string, number>;
    cash?: number;
    total_capital?: number;
    reserved_for_debt?: number;
    liquidity_ratio?: number;
    risk_score?: number;
    expected_return?: number;
  };
  metadata?: Record<string, any>;
}

export interface SearchCompareResponse {
  problem_summary?: Record<string, any>;
  results: Record<string, SearchResult>;
}

export interface OrchestrateAllocationResponse {
  run_id: string;
  status: string;
  ai: StrategyExecutionResponse;
  comparison?: SearchCompareResponse;
  optimization: OptimizationStatusResponse;
}

// User management types (Admin)
export interface UserRecord {
  id: string;
  email: string;
  full_name?: string;
  role: 'admin' | 'analyst' | 'viewer';
}

export interface UpdateRoleInput {
  role: 'admin' | 'analyst' | 'viewer';
}

// Audit log types (Admin)
export interface AuditLogRecord {
  id: string;
  action: string;
  entity_type: string;
  entity_id?: string | null;
  actor: string;
  metadata?: Record<string, any> | null;
  created_at: string;
}

export interface AuditLogsListResponse {
  data: AuditLogRecord[];
  limit: number;
  offset: number;
}

// AI Risk Analysis & Bayesian Network (F5)
export interface BayesianEvidence {
  inflation_high?: boolean;
  rate_hike?: boolean;
  recession_signal?: boolean;
}

export interface BayesianRiskInferenceResult {
  p_recession: number;
  p_liquidity_crisis: number;
  p_equity_downturn: number;
  p_credit_spread_widening: number;
  stress_multiplier: number;
  explanation: string;
}

export interface RiskFactorBreakdown {
  weighted_asset_risk: number;
  concentration_index_hhi: number;
  concentration_penalty: number;
  liquidity_deficit_penalty: number;
  leverage_penalty: number;
  composite_risk_score: number;
  risk_tier: string;
  explanations: string[];
}

export interface MonteCarloSimulationReport {
  num_scenarios: number;
  mean_simulated_return: number;
  std_simulated_return: number;
  var_95: number;
  cvar_95: number;
  var_99: number;
  cvar_99: number;
  worst_case_loss_pct: number;
  probability_of_loss: number;
}

export interface AIRiskAnalyzeResponse {
  risk_breakdown: RiskFactorBreakdown;
  bayesian_inference: BayesianRiskInferenceResult;
  monte_carlo_report?: MonteCarloSimulationReport;
}

export interface AnalyzeTreasuryRiskInput {
  assets: Asset[];
  liabilities?: Liability[];
  constraints?: Constraint[];
  allocations?: Record<string, number>;
  step_size?: number;
  objective?: string;
  market_state?: string;
  evidence?: BayesianEvidence;
  run_monte_carlo?: boolean;
  num_scenarios?: number;
}

// Adversarial Stress Testing & Game Tree Search (F5)
export interface AdversarialStressTestResult {
  worst_case_payoff: number;
  best_treasury_action?: string | null;
  worst_market_shock: string;
  nodes_evaluated: number;
  pruned_branches: number;
  execution_time_ms: number;
  explanation: string;
}

export interface MarketShockScenario {
  shock_name: string;
  equity_drawdown: number;
  credit_spread_spike: number;
  liquidity_freeze: number;
  description: string;
}

export interface ScenarioEvaluation {
  scenario: MarketShockScenario;
  stressed_return: number;
  stressed_liquidity: number;
  stressed_risk: number;
  estimated_payoff: number;
  is_compliant: boolean;
}

export interface TreasuryStressTestResponse {
  adversarial_result: AdversarialStressTestResult;
  scenarios: ScenarioEvaluation[];
}

export interface AdversarialStressTestInput {
  assets: Asset[];
  liabilities?: Liability[];
  constraints?: Constraint[];
  allocations?: Record<string, number>;
  step_size?: number;
  objective?: string;
  market_state?: string;
}

// System Health & Telemetry (Task T-1.1)
export interface SystemHealthResponse {
  status: 'ok' | 'degraded' | string;
  database: 'up' | 'down' | string;
  dependencies?: {
    ai_engine?: 'up' | 'down' | string;
    optimization_engine?: 'up' | 'down' | string;
    [key: string]: string | undefined;
  };
}

// Multi-Agent Council (Task T-3.1)
export type AgentRole = 'treasury' | 'risk' | 'compliance' | 'decision';

export interface FinancialAction {
  action_type: string;
  asset_id?: string;
  liability_id?: string;
  amount: number;
  cost: number;
  reason: string;
}

export interface AgentActionProposal {
  proposed_by: AgentRole;
  action: FinancialAction;
  confidence: number;
  expected_utility_delta: number;
  rationale: string;
}

export interface AgentAssessment {
  agent_name: string;
  role: AgentRole;
  is_acceptable: boolean;
  score: number;
  findings: string[];
  suggested_actions?: AgentActionProposal[];
}

export interface StrategySummary {
  actions: FinancialAction[];
  total_allocated: number;
  expected_return: number;
  risk_score: number;
  liquidity_ratio: number;
  objective_value: number;
  is_compliant: boolean;
  explanation: string;
  metadata?: Record<string, any>;
}

export interface SearchMetrics {
  algorithm: string;
  success: boolean;
  execution_time_ms: number;
  states_explored: number;
  depth: number;
}

export interface CouncilDeliberationSummary {
  treasury_opinion: AgentAssessment;
  risk_opinion: AgentAssessment;
  compliance_opinion: AgentAssessment;
  consensus_reached: boolean;
  recommended_strategy: StrategySummary;
  search_metrics: SearchMetrics;
}

export interface EvaluateCouncilInput {
  objective?: string;
  market_state?: string;
  step_size?: number;
  assets?: Asset[];
  liabilities?: Liability[];
  constraints?: Constraint[];
}


