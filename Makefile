.PHONY: help install test test-ai run-ai run-example docker-up docker-down test-opt run-opt run-opt-example run-opt-benchmark

help:
	@echo "QUANTA Developer Commands:"
	@echo "  make install         - Install AI & Optimization engine Python dependencies"
	@echo "  make test-ai         - Run AI engine test suite (pytest)"
	@echo "  make run-ai          - Start FastAPI AI engine service on port 8000"
	@echo "  make run-example     - Run synthetic ₹100 Cr capital allocation benchmark"
	@echo "  make test-opt        - Run Optimization engine test suite (pytest)"
	@echo "  make run-opt         - Start FastAPI Optimization engine service on port 8001"
	@echo "  make run-opt-example - Run synthetic ₹100 Cr optimization demo comparing solvers"
	@echo "  make run-opt-benchmark - Run full classical-vs-quantum benchmark suite"
	@echo "  make docker-up       - Launch all services with docker-compose"
	@echo "  make docker-down     - Tear down docker-compose services"

install:
	pip install -r services/ai-engine/requirements.txt
	pip install -r services/optimization-engine/requirements.txt

test-ai:
	cd services/ai-engine && pytest -v

run-ai:
	cd services/ai-engine && uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload

run-example:
	python services/ai-engine/examples/basic_capital_allocation.py

test-opt:
	cd services/optimization-engine && pytest -v

run-opt:
	cd services/optimization-engine && uvicorn app.main:app --host 0.0.0.0 --port 8001 --reload

run-opt-example:
	python services/optimization-engine/examples/basic_optimization_demo.py

run-opt-benchmark:
	python -c "import sys; sys.path.insert(0, 'services/optimization-engine'); from app.benchmarks.reporter import generate_benchmark_and_reports; generate_benchmark_and_reports()"

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down

