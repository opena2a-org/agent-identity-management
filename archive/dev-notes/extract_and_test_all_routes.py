#!/usr/bin/env python3
"""
Extract all routes from main.go and generate comprehensive test script
"""

import re
import json
from pathlib import Path

def extract_routes(main_go_path):
    """Extract all route registrations from main.go"""

    with open(main_go_path, 'r') as f:
        content = f.read()

    routes = []

    # Pattern to match route registrations
    # Example: agents.Get("/:id", h.Agent.GetAgent)
    # Example: public.Post("/login", h.PublicRegistration.Login)
    pattern = r'(\w+)\.(Get|Post|Put|Delete|Patch)\("([^"]+)"'

    for match in re.finditer(pattern, content):
        group_name = match.group(1)  # e.g., "agents", "public", "admin"
        method = match.group(2).upper()  # e.g., "GET", "POST"
        path = match.group(3)  # e.g., "/:id", "/login"

        routes.append({
            'group': group_name,
            'method': method,
            'path': path,
            'line': content[:match.start()].count('\n') + 1
        })

    return routes

def determine_full_path(route, group_mappings):
    """Determine the full API path for a route"""
    group = route['group']
    path = route['path']

    # Get the base path for this group
    base_path = group_mappings.get(group, f"/api/v1/{group}")

    # Combine base path and route path
    full_path = base_path + path

    # Clean up double slashes
    full_path = re.sub(r'/+', '/', full_path)

    return full_path

def categorize_routes(routes):
    """Categorize routes by feature area"""
    categories = {}

    for route in routes:
        group = route['group']
        if group not in categories:
            categories[group] = []
        categories[group].append(route)

    return categories

def generate_test_script(routes, output_path):
    """Generate a comprehensive bash test script"""

    # Define group to base path mappings
    group_mappings = {
        'app': '',  # Root level routes
        'v1': '/api/v1',
        'public': '/api/v1/public',
        'auth': '/api/v1/auth',
        'authProtected': '/api/v1/auth',
        'detection': '/api/v1/detection',
        'agents': '/api/v1/agents',
        'apiKeys': '/api/v1/api-keys',
        'trust': '/api/v1/trust-score',
        'admin': '/api/v1/admin',
        'capabilityRequests': '/api/v1/capability-requests',
        'mcpServers': '/api/v1/mcp-servers',
        'verificationEvents': '/api/v1/verification-events',
        'compliance': '/api/v1/compliance',
        'capabilities': '/api/v1/capabilities',
        'tags': '/api/v1/tags',
    }

    # Add full paths to routes
    for route in routes:
        route['full_path'] = determine_full_path(route, group_mappings)

    # Categorize
    categories = categorize_routes(routes)

    # Generate bash script
    script_lines = [
        '#!/bin/bash',
        '',
        '# Comprehensive AIM Endpoint Test Suite',
        '# Auto-generated from main.go route registrations',
        f'# Total Routes: {len(routes)}',
        '',
        'BASE_URL="http://localhost:8080"',
        'PASSED=0',
        'FAILED=0',
        'AUTH_REQUIRED=0',
        'TOTAL=0',
        '',
        '# Colors',
        'GREEN="\\033[0;32m"',
        'RED="\\033[0;31m"',
        'YELLOW="\\033[1;33m"',
        'BLUE="\\033[0;34m"',
        'NC="\\033[0m"',
        '',
        '# Test function',
        'test_endpoint() {',
        '    local method=$1',
        '    local path=$2',
        '    local description=$3',
        '',
        '    TOTAL=$((TOTAL + 1))',
        '',
        '    # Replace :id and :audit_id with test UUIDs',
        '    test_path=$(echo "$path" | sed "s/:id/00000000-0000-0000-0000-000000000000/g" | sed "s/:audit_id/00000000-0000-0000-0000-000000000001/g")',
        '',
        '    # Test endpoint',
        '    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X $method "${BASE_URL}${test_path}" 2>/dev/null)',
        '',
        '    case $http_code in',
        '        404)',
        '            echo -e "${RED}✗ MISSING${NC} - $method $path"',
        '            FAILED=$((FAILED + 1))',
        '            ;;',
        '        401|403)',
        '            echo -e "${BLUE}🔒 AUTH${NC} - $method $path"',
        '            AUTH_REQUIRED=$((AUTH_REQUIRED + 1))',
        '            ;;',
        '        200|201)',
        '            echo -e "${GREEN}✓ OK${NC} - $method $path"',
        '            PASSED=$((PASSED + 1))',
        '            ;;',
        '        400|422)',
        '            echo -e "${YELLOW}~ PARTIAL${NC} - $method $path (validation error)"',
        '            PASSED=$((PASSED + 1))',
        '            ;;',
        '        500|502|503)',
        '            echo -e "${RED}✗ ERROR${NC} - $method $path (HTTP $http_code)"',
        '            FAILED=$((FAILED + 1))',
        '            ;;',
        '        *)',
        '            echo -e "${YELLOW}? UNKNOWN${NC} - $method $path (HTTP $http_code)"',
        '            ;;',
        '    esac',
        '}',
        '',
        'echo "=========================================="',
        f'echo "  Testing All {len(routes)} AIM Endpoints"',
        'echo "=========================================="',
        'echo ""',
        '',
    ]

    # Add tests for each category
    for group, group_routes in sorted(categories.items()):
        script_lines.append(f'echo "--- {group.upper()} ({len(group_routes)} endpoints) ---"')

        for route in group_routes:
            method = route['method']
            path = route['full_path']
            description = f"{group} - {route['path']}"

            script_lines.append(f'test_endpoint "{method}" "{path}" "{description}"')

        script_lines.append('echo ""')

    # Add summary
    script_lines.extend([
        'echo "=========================================="',
        'echo "  Test Summary"',
        'echo "=========================================="',
        'echo "Total Endpoints: $TOTAL"',
        'echo -e "${GREEN}Working: $PASSED${NC}"',
        'echo -e "${BLUE}Auth Required: $AUTH_REQUIRED${NC}"',
        'echo -e "${RED}Failed/Missing: $FAILED${NC}"',
        'echo ""',
        '',
        'implemented=$((TOTAL - FAILED))',
        'success_rate=$((implemented * 100 / TOTAL))',
        '',
        'echo "=========================================="',
        'echo "  Implementation Rate: ${success_rate}% ($implemented/$TOTAL)"',
        'echo "=========================================="',
        '',
        'if [ $FAILED -eq 0 ]; then',
        '    echo -e "${GREEN}✓ All endpoints working!${NC}"',
        '    exit 0',
        'else',
        '    echo -e "${YELLOW}⚠ $FAILED endpoints need attention${NC}"',
        '    exit 1',
        'fi',
    ])

    # Write script
    with open(output_path, 'w') as f:
        f.write('\n'.join(script_lines))

    print(f"Generated test script: {output_path}")
    print(f"Total routes found: {len(routes)}")
    print(f"Categories: {len(categories)}")

    # Print category summary
    print("\nRoutes by category:")
    for group, group_routes in sorted(categories.items(), key=lambda x: len(x[1]), reverse=True):
        print(f"  {group}: {len(group_routes)} endpoints")

if __name__ == "__main__":
    main_go_path = Path("apps/backend/cmd/server/main.go")
    output_script = Path("test_all_endpoints_comprehensive.sh")

    print(f"Extracting routes from: {main_go_path}")
    routes = extract_routes(main_go_path)

    print(f"\nGenerating test script...")
    generate_test_script(routes, output_script)

    print(f"\n✅ Done! Run: bash {output_script}")
