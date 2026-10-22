"""Pydantic v2 domain models for financial problem specification."""

from __future__ import annotations
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field, model_validator


class Asset(BaseModel):
    """Investment asset specification.
    
    Required contract fields: name, cost, expected_return, risk, liquidity.
    """
    name: str = Field(..., description="Asset identifier / name")
    cost: float = Field(default=0.0, ge=0.0, description="Acquisition / transaction cost (Rs Cr or fraction)")
    expected_return: float = Field(..., ge=-1.0, le=5.0, description="Annualized expected return fraction")
    risk: float = Field(..., ge=0.0, le=2.0, description="Normalized risk / volatility index")
    liquidity: float = Field(..., ge=0.0, le=1.0, description="Normalized liquidity factor in [0, 1]")
    
    # Optional interoperability fields
    id: Optional[str] = Field(default=None, description="Unique asset ID (defaults to name)")
    asset_type: Optional[str] = Field(default="other", description="Asset class type")
    volatility: Optional[float] = Field(default=0.05, ge=0.0, description="Annual return volatility")
    min_investment: Optional[float] = Field(default=0.0, ge=0.0, description="Minimum allocation limit")
    max_investment: Optional[float] = Field(default=None, ge=0.0, description="Maximum allocation limit")
    group: Optional[str] = Field(default=None, description="Asset sector or grouping tag")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Arbitrary annotations")

    @model_validator(mode="after")
    def populate_id(self) -> Asset:
        if not self.id:
            self.id = self.name
        return self


class Liability(BaseModel):
    """Corporate debt or liability obligation.
    
    Required contract fields: amount, interest_rate, maturity.
    """
    amount: float = Field(default=0.0, ge=0.0, description="Obligation principal amount in Rs Cr")
    interest_rate: float = Field(default=0.0, ge=0.0, le=2.0, description="Annual interest coupon fraction")
    maturity: int = Field(default=12, ge=0, description="Time to maturity in months")
    
    # Optional interoperability fields
    id: Optional[str] = Field(default=None, description="Unique liability ID")
    name: Optional[str] = Field(default="liability", description="Obligation description")
    principal: Optional[float] = Field(default=None, description="Alias for amount")
    maturity_months: Optional[int] = Field(default=None, description="Alias for maturity")
    priority: int = Field(default=1, ge=1, description="Seniority priority level")
    is_mandatory_reserve: bool = Field(default=True, description="Whether liquidity must be reserved")

    @model_validator(mode="before")
    @classmethod
    def normalize_aliases(cls, data: Any) -> Any:
        if isinstance(data, dict):
            if "principal" in data and ("amount" not in data or data.get("amount") is None):
                data["amount"] = data["principal"]
            if "amount" in data and ("principal" not in data or data.get("principal") is None):
                data["principal"] = data["amount"]
            if "maturity_months" in data and ("maturity" not in data or data.get("maturity") is None):
                data["maturity"] = data["maturity_months"]
            if "maturity" in data and ("maturity_months" not in data or data.get("maturity_months") is None):
                data["maturity_months"] = data["maturity"]
        return data


class Constraint(BaseModel):
    """Compliance rule or allocation limit constraint."""
    id: Optional[str] = Field(default=None, description="Constraint identifier")
    constraint_type: str = Field(..., description="Type of constraint (e.g. minimum_liquidity, budget)")
    description: Optional[str] = Field(default="", description="Policy description")
    parameters: Dict[str, Any] = Field(default_factory=dict, description="Numerical thresholds")
    severity: Optional[str] = Field(default="critical", description="Severity level")
    penalty_weight: float = Field(default=10.0, ge=0.0, description="Penalty weight for soft constraints")


class FinancialProblem(BaseModel):
    """Canonical specification of a corporate capital optimization problem.
    
    All contract fields required:
        capital, assets, liabilities, constraints, risk_tolerance, minimum_liquidity, objective.
    """
    capital: float = Field(..., gt=0.0, description="Total balance sheet capital (Rs Cr)")
    assets: List[Asset] = Field(..., description="Eligible investment assets")
    liabilities: List[Liability] = Field(default_factory=list, description="Corporate liabilities")
    constraints: List[Constraint] = Field(default_factory=list, description="Policy constraints")
    risk_tolerance: float = Field(default=0.50, ge=0.0, le=1.0, description="Risk tolerance in [0, 1]")
    minimum_liquidity: float = Field(default=0.20, ge=0.0, le=1.0, description="Liquidity floor fraction")
    objective: str = Field(default="maximize_risk_adjusted_return", description="Target objective")

    # Optional extension fields
    step_size: Optional[float] = Field(default=1.0, gt=0.0, description="Discretization step size (Rs Cr)")
    max_exposure_pct: Optional[float] = Field(default=None, ge=0.0, le=1.0, description="Max exposure per asset")
    group_caps: Optional[Dict[str, float]] = Field(default=None, description="Asset class / sector caps")
    covariance_matrix: Optional[List[List[float]]] = Field(default=None, description="Explicit covariance matrix")
    candidate_subset: Optional[List[str]] = Field(default=None, description="Restricted asset IDs from AI engine")
    market_state: Optional[str] = Field(default="normal", description="Market regime indicator")

