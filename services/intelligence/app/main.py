from __future__ import annotations

import uuid
from fastapi import FastAPI, Request
from fastapi.encoders import jsonable_encoder
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from app.forecasting import run_forecast
from app.optimizer import run_optimization
from app.risk import run_risk_analysis
from app.schemas import (
    ForecastRequest,
    ForecastResponse,
    MODEL_SERVICE_VERSION,
    OptimizeRequest,
    OptimizeResponse,
    RiskRequest,
    RiskResponse,
)


app = FastAPI(
    title="QUANTA Intelligence Service",
    version=MODEL_SERVICE_VERSION,
    description="Stateless, non-authoritative forecasting, risk simulation, and LP/MILP optimization service.",
)


@app.middleware("http")
async def correlation_id_middleware(request: Request, call_next):
    corr_id = request.headers.get("X-Correlation-ID", "").strip() or str(uuid.uuid4())
    request.state.correlation_id = corr_id
    response = await call_next(request)
    response.headers["X-Correlation-ID"] = corr_id
    return response


@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    corr_id = getattr(request.state, "correlation_id", str(uuid.uuid4()))
    return JSONResponse(
        status_code=422,
        content={
            "error": {
                "code": "INVALID_CONTRACT_PAYLOAD",
                "message": "request failed schema validation",
                "correlation_id": corr_id,
                "details": jsonable_encoder(exc.errors()),
            }
        },
        headers={"X-Correlation-ID": corr_id},
    )


@app.exception_handler(ValueError)
async def value_error_handler(request: Request, exc: ValueError):
    corr_id = getattr(request.state, "correlation_id", str(uuid.uuid4()))
    return JSONResponse(
        status_code=400,
        content={
            "error": {
                "code": "VALIDATION_ERROR",
                "message": str(exc),
                "correlation_id": corr_id,
            }
        },
        headers={"X-Correlation-ID": corr_id},
    )


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "service": "quanta-intelligence", "version": MODEL_SERVICE_VERSION}


@app.get("/ready")
def ready() -> dict[str, str]:
    return {"status": "ready", "service": "quanta-intelligence", "version": MODEL_SERVICE_VERSION}


@app.post("/internal/v1/forecast", response_model=ForecastResponse)
def forecast(req: ForecastRequest) -> ForecastResponse:
    return run_forecast(req)


@app.post("/internal/v1/risk/evaluate", response_model=RiskResponse)
def evaluate_risk(req: RiskRequest) -> RiskResponse:
    return run_risk_analysis(req)


@app.post("/internal/v1/optimize", response_model=OptimizeResponse)
def optimize(req: OptimizeRequest) -> OptimizeResponse:
    return run_optimization(req)
