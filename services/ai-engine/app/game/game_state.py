"""Adversarial stress-testing game state between Treasury Optimizer (MAX) and Market Adversary (MIN)."""

from typing import Dict, List, Optional
from pydantic import BaseModel, Field
from app.domain.asset import Asset
from app.domain.financial_state import FinancialState
from app.domain.strategy import FinancialAction


class MarketShockAction(BaseModel):
    """Adversarial shock executed by market environment (MIN player)."""
    shock_name: str
    equity_drawdown: float = 0.0  # e.g. -0.25 (-25%)
    credit_spread_spike: float = 0.0  # e.g. -0.05
    liquidity_freeze: float = 0.0  # e.g. -0.20
    description: str = ""


class TreasuryGameState:
    """State node in the Treasury vs Adverse Market game tree."""

    def __init__(
        self,
        financial_state: FinancialState,
        assets_map: Dict[str, Asset],
        is_max_turn: bool = True,
        active_shock: Optional[MarketShockAction] = None,
        depth: int = 0
    ):
        self.financial_state = financial_state
        self.assets_map = assets_map
        self.is_max_turn = is_max_turn
        self.active_shock = active_shock
        self.depth = depth

    def is_terminal(self, max_depth: int = 4) -> bool:
        """Check if game state has reached terminal depth or fully allocated state."""
        if self.depth >= max_depth:
            return True
        return self.financial_state.cash < 5.0 and not self.is_max_turn

    def evaluate_payoff(self, risk_tolerance: float = 0.50) -> float:
        """Utility payoff function for the MAX player (Treasury) under adversarial shocks."""
        st = self.financial_state
        base_return = st.expected_return
        base_risk = st.risk_score
        base_liq = st.liquidity_ratio

        # Apply market shock penalties to payoff
        if self.active_shock:
            # Impact on return: sum allocations by asset_type (IDs may differ from type name)
            equity_exposure = 0.0
            bond_exposure = 0.0
            for a_id, amount in st.allocations.items():
                asset = self.assets_map.get(a_id)
                if asset is None:
                    # Graceful fallback when asset_id is missing from assets_map
                    atype = a_id.lower()
                else:
                    atype = (
                        asset.asset_type.value
                        if hasattr(asset.asset_type, "value")
                        else str(asset.asset_type)
                    ).lower()
                if atype == "equity":
                    equity_exposure += amount
                elif atype in ("corp_bond", "corporate_bond", "bond"):
                    bond_exposure += amount
            shock_return_loss = (
                equity_exposure * abs(self.active_shock.equity_drawdown) +
                bond_exposure * abs(self.active_shock.credit_spread_spike)
            ) / max(1.0, st.total_capital)
            
            stressed_return = base_return - shock_return_loss
            stressed_liq = max(0.0, base_liq - abs(self.active_shock.liquidity_freeze))
            stressed_risk = min(1.0, base_risk * 1.4)
            
            payoff = stressed_return - (1.0 - risk_tolerance) * 1.5 * (stressed_risk ** 2) + 0.20 * stressed_liq
            return float(payoff)

        return float(st.utility_score(risk_tolerance))
