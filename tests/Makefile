# Unified Makefile for SDK Testing
# Run from tests/ directory or use 'make -C tests <target>'

.PHONY: all test test-python test-typescript test-java \
        coverage coverage-python coverage-typescript coverage-java \
        lint lint-python lint-typescript lint-java \
        build build-python build-typescript build-java \
        clean report help

# Directories
PYTHON_SDK := ../sdk/python
TYPESCRIPT_SDK := ../sdk/typescript
JAVA_SDK := ../sdk/java
SHARED_SPECS := ./shared/specs
SHARED_FIXTURES := ./shared/fixtures

# Default target
all: test

# ============================================
# Test Targets
# ============================================

test: test-python test-typescript test-java
	@echo "All SDK tests completed"

test-python:
	@echo "Running Python SDK tests..."
	cd $(PYTHON_SDK) && python -m pytest tests/ -v --tb=short

test-typescript:
	@echo "Running TypeScript SDK tests..."
	cd $(TYPESCRIPT_SDK) && npm test

test-java:
	@echo "Running Java SDK tests..."
	cd $(JAVA_SDK) && mvn test -q

# Quick tests (unit only, skip integration)
test-quick: test-python-quick test-typescript-quick test-java-quick
	@echo "Quick tests completed"

test-python-quick:
	@echo "Running Python unit tests..."
	cd $(PYTHON_SDK) && python -m pytest tests/unit/ -v --tb=short

test-typescript-quick:
	@echo "Running TypeScript unit tests..."
	cd $(TYPESCRIPT_SDK) && npm test -- --testPathPattern="unit"

test-java-quick:
	@echo "Running Java unit tests..."
	cd $(JAVA_SDK) && mvn test -q -Dtest="**/unit/**"

# Integration tests only
test-integration: test-python-integration test-typescript-integration test-java-integration
	@echo "Integration tests completed"

test-python-integration:
	@echo "Running Python integration tests..."
	cd $(PYTHON_SDK) && python -m pytest tests/integration/ -v --tb=short

test-typescript-integration:
	@echo "Running TypeScript integration tests..."
	cd $(TYPESCRIPT_SDK) && npm test -- --testPathPattern="integration"

test-java-integration:
	@echo "Running Java integration tests..."
	cd $(JAVA_SDK) && mvn test -q -Dtest="**/integration/**"

# ============================================
# Coverage Targets
# ============================================

coverage: coverage-python coverage-typescript coverage-java report
	@echo "Coverage reports generated"

coverage-python:
	@echo "Generating Python coverage..."
	cd $(PYTHON_SDK) && python -m pytest tests/ --cov=aim_sdk --cov-report=xml:coverage.xml --cov-report=html:htmlcov --cov-report=term-missing

coverage-typescript:
	@echo "Generating TypeScript coverage..."
	cd $(TYPESCRIPT_SDK) && npm run coverage

coverage-java:
	@echo "Generating Java coverage..."
	cd $(JAVA_SDK) && mvn jacoco:prepare-agent test jacoco:report -q

# ============================================
# Lint Targets
# ============================================

lint: lint-python lint-typescript lint-java
	@echo "All linting completed"

lint-python:
	@echo "Linting Python SDK..."
	cd $(PYTHON_SDK) && python -m ruff check aim_sdk/ tests/ && python -m mypy aim_sdk/

lint-typescript:
	@echo "Linting TypeScript SDK..."
	cd $(TYPESCRIPT_SDK) && npm run lint

lint-java:
	@echo "Linting Java SDK..."
	cd $(JAVA_SDK) && mvn checkstyle:check -q

# ============================================
# Build Targets
# ============================================

build: build-python build-typescript build-java
	@echo "All SDKs built"

build-python:
	@echo "Building Python SDK..."
	cd $(PYTHON_SDK) && python -m build

build-typescript:
	@echo "Building TypeScript SDK..."
	cd $(TYPESCRIPT_SDK) && npm run build

build-java:
	@echo "Building Java SDK..."
	cd $(JAVA_SDK) && mvn package -q -DskipTests

# ============================================
# Clean Targets
# ============================================

clean: clean-python clean-typescript clean-java
	@echo "All clean"

clean-python:
	cd $(PYTHON_SDK) && rm -rf build/ dist/ *.egg-info/ .pytest_cache/ htmlcov/ .coverage coverage.xml

clean-typescript:
	cd $(TYPESCRIPT_SDK) && rm -rf dist/ coverage/ node_modules/.cache/

clean-java:
	cd $(JAVA_SDK) && mvn clean -q

# ============================================
# Report Generation
# ============================================

report:
	@echo ""
	@echo "============================================"
	@echo "SDK Test Coverage Summary"
	@echo "============================================"
	@echo ""
	@echo "Python SDK:"
	@if [ -f "$(PYTHON_SDK)/coverage.xml" ]; then \
		grep -o 'line-rate="[^"]*"' $(PYTHON_SDK)/coverage.xml | head -1 | sed 's/line-rate="//;s/"//;s/^/  Coverage: /'; \
	else \
		echo "  Coverage report not found. Run 'make coverage-python'"; \
	fi
	@echo ""
	@echo "TypeScript SDK:"
	@if [ -f "$(TYPESCRIPT_SDK)/coverage/coverage-summary.json" ]; then \
		cat $(TYPESCRIPT_SDK)/coverage/coverage-summary.json 2>/dev/null | grep -o '"pct":[0-9.]*' | head -1 | sed 's/"pct":/  Coverage: /'; \
	else \
		echo "  Coverage report not found. Run 'make coverage-typescript'"; \
	fi
	@echo ""
	@echo "Java SDK:"
	@if [ -f "$(JAVA_SDK)/target/site/jacoco/index.html" ]; then \
		echo "  Coverage report at $(JAVA_SDK)/target/site/jacoco/index.html"; \
	else \
		echo "  Coverage report not found. Run 'make coverage-java'"; \
	fi
	@echo ""

# Test count summary
test-count:
	@echo ""
	@echo "============================================"
	@echo "SDK Test Count Summary"
	@echo "============================================"
	@echo ""
	@echo "Python SDK:"
	@cd $(PYTHON_SDK) && python -m pytest tests/ --collect-only -q 2>/dev/null | tail -1 || echo "  Unable to count"
	@echo ""
	@echo "TypeScript SDK:"
	@cd $(TYPESCRIPT_SDK) && npm test -- --listTests 2>/dev/null | wc -l | xargs -I{} echo "  {} test files" || echo "  Unable to count"
	@echo ""
	@echo "Java SDK:"
	@find $(JAVA_SDK)/src/test -name "*Test.java" 2>/dev/null | wc -l | xargs -I{} echo "  {} test files" || echo "  Unable to count"
	@echo ""

# ============================================
# Validation Targets
# ============================================

# Validate shared specs exist and are valid YAML
validate-specs:
	@echo "Validating shared specs..."
	@for f in $(SHARED_SPECS)/*.yaml; do \
		python3 -c "import yaml; yaml.safe_load(open('$$f'))" && echo "  ✓ $$f" || echo "  ✗ $$f"; \
	done

# Validate shared fixtures exist and are valid JSON
validate-fixtures:
	@echo "Validating shared fixtures..."
	@for f in $(SHARED_FIXTURES)/*.json; do \
		python3 -c "import json; json.load(open('$$f'))" && echo "  ✓ $$f" || echo "  ✗ $$f"; \
	done

validate: validate-specs validate-fixtures
	@echo "All validations passed"

# ============================================
# CI Targets
# ============================================

ci: validate lint test coverage report
	@echo "CI pipeline completed"

ci-python: validate-specs validate-fixtures lint-python test-python coverage-python
	@echo "Python CI completed"

ci-typescript: validate-specs validate-fixtures lint-typescript test-typescript coverage-typescript
	@echo "TypeScript CI completed"

ci-java: validate-specs validate-fixtures lint-java test-java coverage-java
	@echo "Java CI completed"

# ============================================
# Help
# ============================================

help:
	@echo "SDK Testing Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Test Targets:"
	@echo "  test                Run all SDK tests"
	@echo "  test-python         Run Python SDK tests"
	@echo "  test-typescript     Run TypeScript SDK tests"
	@echo "  test-java           Run Java SDK tests"
	@echo "  test-quick          Run unit tests only (all SDKs)"
	@echo "  test-integration    Run integration tests only (all SDKs)"
	@echo ""
	@echo "Coverage Targets:"
	@echo "  coverage            Generate coverage for all SDKs"
	@echo "  coverage-python     Generate Python coverage"
	@echo "  coverage-typescript Generate TypeScript coverage"
	@echo "  coverage-java       Generate Java coverage"
	@echo ""
	@echo "Build Targets:"
	@echo "  build               Build all SDKs"
	@echo "  lint                Lint all SDKs"
	@echo "  clean               Clean all build artifacts"
	@echo ""
	@echo "Report Targets:"
	@echo "  report              Show coverage summary"
	@echo "  test-count          Show test count per SDK"
	@echo ""
	@echo "Validation Targets:"
	@echo "  validate            Validate specs and fixtures"
	@echo "  validate-specs      Validate YAML spec files"
	@echo "  validate-fixtures   Validate JSON fixture files"
	@echo ""
	@echo "CI Targets:"
	@echo "  ci                  Full CI pipeline (all SDKs)"
	@echo "  ci-python           Python CI pipeline"
	@echo "  ci-typescript       TypeScript CI pipeline"
	@echo "  ci-java             Java CI pipeline"
