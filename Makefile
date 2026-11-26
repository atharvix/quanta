.PHONY: help install start-local stop-local smoke-test init-db test test-ai test-opt test-backend test-frontend run-ai run-opt run-backend run-frontend run-example run-opt-example run-opt-benchmark docker-up docker-down

help:
	@echo "QUANTA Native Local Developer Commands (Recommended):"
	@echo "  make start-local       - Launch PostgreSQL check + AI Engine, Opt Engine, Go Backend & Frontend natively"
	@echo "  make stop-local        - Stop native QUANTA processes on ports 8000, 8001, 8080, 3000"
	@echo "  make smoke-test        - Run end-to-end health, auth, optimization, and stress-test verification"
	@echo "  make init-db           - Run scripts/init_db.sql using psql"
	@echo "  make install           - Install AI Engine, Optimization Engine & Frontend dependencies"
	@echo "  make test              - Run AI Engine, Optimization Engine, Go Backend & Frontend unit tests"
	@echo "  make run-ai            - Start FastAPI AI Engine on port 8000"
	@echo "  make run-opt           - Start FastAPI Optimization Engine on port 8001"
	@echo "  make run-backend       - Start Go API Gateway on port 8080"
	@echo "  make run-frontend      - Start SvelteKit dev server on port 3000"
	@echo "  make run-example       - Run synthetic ₹100 Cr capital allocation benchmark"
	@echo "  make run-opt-example   - Run synthetic ₹100 Cr optimization demo comparing solvers"
	@echo "  make run-opt-benchmark - Run full classical-vs-quantum benchmark suite"
	@echo ""
	@echo "Optional Container Workflow:"
	@echo "  make docker-up         - (Optional) Launch all services with Docker Compose"
	@echo "  make docker-down       - (Optional) Tear down Docker Compose services"

install:
	pip install -r services/ai-engine/requirements.txt
	pip install -r services/optimization-engine/requirements.txt
	cd apps/frontend && npm install

init-db:
	psql -f scripts/init_db.sql

start-local:
	./scripts/start_quanta.sh

stop-local:
	./scripts/stop_quanta.sh

smoke-test:
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke_test.ps1

test-ai:
	cd services/ai-engine && pytest -v

test-opt:
	cd services/optimization-engine && pytest -v

test-backend:
	cd apps/backend && go test ./...

test-frontend:
	cd apps/frontend && npm test

test: test-ai test-opt test-backend test-frontend

run-ai:
	cd services/ai-engine && uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload

run-opt:
	cd services/optimization-engine && uvicorn app.main:app --host 0.0.0.0 --port 8001 --reload

run-backend:
	cd apps/backend && go run ./cmd/server

run-frontend:
	cd apps/frontend && npm run dev -- --port 3000

run-example:
	python services/ai-engine/examples/basic_capital_allocation.py

run-opt-example:
	python services/optimization-engine/examples/basic_optimization_demo.py

run-opt-benchmark:
	python -c "import sys; sys.path.insert(0, 'services/optimization-engine'); from app.benchmarks.reporter import generate_benchmark_and_reports; generate_benchmark_and_reports()"

docker-up:
	docker compose up --build

docker-down:
	docker compose down
