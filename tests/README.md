# SDK Testing Infrastructure

This directory contains shared testing infrastructure for all AIM SDKs (Python, TypeScript, Java).

## Directory Structure

```
tests/
├── Makefile                  # Unified test commands for all SDKs
├── README.md                 # This file
└── shared/
    ├── specs/                # Behavior specifications (YAML)
    │   ├── client-behavior.yaml
    │   ├── error-handling.yaml
    │   ├── authentication.yaml
    │   └── a2a-protocol.yaml
    └── fixtures/             # Test data fixtures (JSON)
        ├── agents.json
        ├── verification.json
        ├── a2a.json
        └── errors.json
```

## Quick Start

```bash
# Run all SDK tests
make -C tests test

# Run tests for specific SDK
make -C tests test-python
make -C tests test-typescript
make -C tests test-java

# Generate coverage reports
make -C tests coverage

# View test counts
make -C tests test-count
```

## Behavior Specifications

The `shared/specs/` directory contains YAML files that define expected behaviors across all SDKs. These specifications ensure consistency in:

- **client-behavior.yaml**: Core client initialization, authentication, and API interactions
- **error-handling.yaml**: Error classes, HTTP error mapping, and exception handling
- **authentication.yaml**: OAuth flows, credential management, and cryptography
- **a2a-protocol.yaml**: Agent-to-Agent protocol behaviors

### Using Specs in Tests

Each SDK should implement tests that verify the behaviors defined in these specs:

**Python**:
```python
import yaml
import json

def load_spec(name):
    with open(f'../../tests/shared/specs/{name}.yaml') as f:
        return yaml.safe_load(f)

def load_fixture(name):
    with open(f'../../tests/shared/fixtures/{name}.json') as f:
        return json.load(f)
```

**TypeScript**:
```typescript
import { readFileSync } from 'fs';
import { parse } from 'yaml';

const loadSpec = (name: string) =>
  parse(readFileSync(`../../tests/shared/specs/${name}.yaml`, 'utf8'));

const loadFixture = (name: string) =>
  JSON.parse(readFileSync(`../../tests/shared/fixtures/${name}.json`, 'utf8'));
```

**Java**:
```java
import com.fasterxml.jackson.databind.ObjectMapper;
import org.yaml.snakeyaml.Yaml;

public class TestFixtures {
    public static Map<String, Object> loadSpec(String name) {
        Yaml yaml = new Yaml();
        return yaml.load(new FileInputStream("../../tests/shared/specs/" + name + ".yaml"));
    }

    public static JsonNode loadFixture(String name) {
        ObjectMapper mapper = new ObjectMapper();
        return mapper.readTree(new File("../../tests/shared/fixtures/" + name + ".json"));
    }
}
```

## Test Fixtures

The `shared/fixtures/` directory contains JSON test data that should be used consistently across SDKs:

- **agents.json**: Agent registration requests/responses, agent states
- **verification.json**: Action verification requests/responses, trust thresholds
- **a2a.json**: Agent cards, skills, peer trust, consents, security settings
- **errors.json**: API error responses, error class definitions

## Coverage Targets

| SDK        | Target | Current |
|------------|--------|---------|
| Python     | 80%    | -       |
| TypeScript | 80%    | -       |
| Java       | 80%    | -       |

Run `make -C tests coverage` to generate current coverage reports.

## Test Categories

### Unit Tests
- Test individual classes and functions
- Mock all external dependencies
- Fast execution (<1 second per test)

### Integration Tests
- Test SDK against mock server or test environment
- Verify HTTP client behavior
- Test error handling with real responses

### Contract Tests
- Verify SDK matches shared specifications
- Test type validation against specs
- Ensure cross-SDK consistency

## CI Integration

The Makefile provides CI-ready targets:

```bash
# Full CI pipeline for all SDKs
make -C tests ci

# Individual SDK CI
make -C tests ci-python
make -C tests ci-typescript
make -C tests ci-java
```

Each CI target runs:
1. Validate specs and fixtures
2. Lint code
3. Run tests
4. Generate coverage

## Adding New Behaviors

1. Add behavior to appropriate spec file in `shared/specs/`
2. Add test fixtures to `shared/fixtures/` if needed
3. Implement tests in each SDK
4. Verify with `make -C tests validate`

## Troubleshooting

### Specs not loading
Ensure you're running tests from the SDK directory, not the tests directory.

### Missing dependencies
```bash
# Python
pip install pyyaml pytest pytest-cov

# TypeScript
npm install yaml

# Java - dependencies in pom.xml
```

### Coverage reports not generating
```bash
# Python
pip install pytest-cov

# TypeScript - configure in vitest.config.ts
# Java - JaCoCo plugin in pom.xml
```
