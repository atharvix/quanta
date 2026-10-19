"""Configuration settings for QUANTA Optimization Engine."""

import os
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Optimization Engine settings loaded from environment."""

    APP_NAME: str = "QUANTA Optimization Engine"
    ENVIRONMENT: str = "development"
    LOG_LEVEL: str = "INFO"
    DEBUG: bool = True
    OPTIMIZATION_ENGINE_HOST: str = "0.0.0.0"
    OPTIMIZATION_ENGINE_PORT: int = 8001

    # Quantum & Optimization Defaults
    MAX_QUBITS_CAP: int = 20  # Hard cap on simulator qubit count
    DEFAULT_DISCRETIZATION_UNIT: float = 1.0  # Rs 1 Cr granularity for slack variables
    DEFAULT_PENALTY_MULTIPLIER: float = 10.0
    DEFAULT_QAOA_SHOTS: int = 2048

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )


settings = Settings()

