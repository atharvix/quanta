"""Domain model representing corporate liabilities and obligations."""

from pydantic import BaseModel, Field, field_validator


class Liability(BaseModel):
    """Corporate liability or debt obligation impacting treasury liquidity.
    
    Attributes:
        id: Unique identifier for the obligation.
        name: Name or description (e.g. 'Term Loan A', 'Working Capital Facility').
        principal: Outstanding principal amount (e.g. ₹15 Cr).
        interest_rate: Annual coupon or interest rate (e.g. 0.085 for 8.5%).
        maturity_months: Time remaining until maturity in months.
        priority: Seniority level (1 = highest priority / senior debt).
        is_mandatory_reserve: Flag requiring liquid capital earmarked for debt servicing.
    """
    id: str = Field(..., description="Unique liability identifier")
    name: str = Field(..., description="Liability display name")
    principal: float = Field(..., ge=0.0, description="Principal debt amount")
    interest_rate: float = Field(..., ge=0.0, le=2.0, description="Annualized interest cost")
    maturity_months: int = Field(default=12, ge=1, description="Remaining maturity in months")
    priority: int = Field(default=1, ge=1, description="Seniority rank (1 = Senior, 2 = Mezzanine, etc.)")
    is_mandatory_reserve: bool = Field(default=True, description="Whether reserve allocation is mandatory")

    @field_validator("name")
    @classmethod
    def validate_name_not_empty(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("Liability name cannot be empty")
        return v
