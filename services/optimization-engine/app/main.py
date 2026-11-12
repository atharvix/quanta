"""Main application entry point for QUANTA Optimization Engine."""

import logging
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from app.config import settings
from app.api.routes import router as api_router

# Configure logging
logging.basicConfig(
    level=getattr(logging, settings.LOG_LEVEL.upper(), logging.INFO),
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("optimization_engine")

app = FastAPI(
    title="QUANTA Optimization Engine",
    description=(
        "Autonomous Corporate Treasury Capital Allocation Optimization Service. "
        "Mathematical Programming (LP/QP), Quadratic Unconstrained Binary Optimization (QUBO), "
        "and Quantum Approximate Optimization Algorithm (QAOA) Simulation."
    ),
    version="0.1.0",
    docs_url="/docs",
    redoc_url="/redoc",
)

# CORS middleware for Go backend and frontend
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health", tags=["Health"])
async def health_check():
    """Health check endpoint for container orchestrators and backend gateway."""
    return {
        "status": "healthy",
        "service": "quanta-optimization-engine",
        "environment": settings.ENVIRONMENT,
        "version": "0.1.0",
        "port": settings.OPTIMIZATION_ENGINE_PORT,
    }


# Mount optimization routes
app.include_router(api_router)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "app.main:app",
        host=settings.OPTIMIZATION_ENGINE_HOST,
        port=settings.OPTIMIZATION_ENGINE_PORT,
        reload=settings.DEBUG,
    )

