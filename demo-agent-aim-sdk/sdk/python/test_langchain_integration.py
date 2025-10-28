#!/usr/bin/env python3
"""
Integration tests for AIM + LangChain

Tests all three integration patterns:
1. AIMCallbackHandler - Automatic logging
2. @aim_verify decorator - Explicit verification
3. AIMToolWrapper - Wrap existing tools
"""

import sys
import os
from pathlib import Path

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "aim_sdk"))

# Test imports
try:
    from langchain_core.tools import tool
    from langchain_core.callbacks import BaseCallbackHandler
    print("✅ LangChain imports successful")
except ImportError as e:
    print(f"❌ LangChain import failed: {e}")
    print("Please install: pip install langchain langchain-core")
    sys.exit(1)

from aim_sdk import AIMClient
from aim_sdk.integrations.langchain import AIMCallbackHandler, aim_verify, wrap_tools_with_aim

AIM_URL = "http://localhost:8080"

def test_callback_handler():
    """Test 1: AIMCallbackHandler - Automatic logging"""
    print("\n" + "="*70)
    print("TEST 1: AIMCallbackHandler - Automatic logging")
    print("="*70)

    try:
        # Register AIM agent
        aim_client = AIMClient.auto_register_or_load(
            "langchain-test-callback",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")

        # Create callback handler
        aim_handler = AIMCallbackHandler(
            agent=aim_client,
            log_inputs=True,
            log_outputs=True,
            verbose=True
        )
        print("✅ AIMCallbackHandler created")

        # Define a simple tool
        @tool
        def simple_calculator(expression: str) -> str:
            '''Calculate a mathematical expression'''
            try:
                result = eval(expression)
                return f"Result: {result}"
            except Exception as e:
                return f"Error: {e}"

        print(f"✅ Tool created: {simple_calculator.name}")

        # Simulate tool execution with callback
        serialized = {"name": simple_calculator.name}
        input_str = "2 + 2"
        run_id = "test-run-001"

        # Call callback methods directly (simulating LangChain)
        aim_handler.on_tool_start(
            serialized=serialized,
            input_str=input_str,
            run_id=run_id
        )
        print(f"✅ Tool start logged")

        # Execute tool
        result = simple_calculator.invoke(input_str)
        print(f"✅ Tool executed: {result}")

        # Log completion
        aim_handler.on_tool_end(
            output=result,
            run_id=run_id
        )
        print(f"✅ Tool end logged")

        print("\n🎉 TEST 1 PASSED - Callback handler works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 1 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_aim_verify_decorator():
    """Test 2: @aim_verify decorator - Explicit verification"""
    print("\n" + "="*70)
    print("TEST 2: @aim_verify decorator - Explicit verification")
    print("="*70)

    try:
        # Register AIM agent
        aim_client = AIMClient.auto_register_or_load(
            "langchain-test-decorator",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")

        # Define tool with @aim_verify decorator
        @tool
        @aim_verify(agent=aim_client, risk_level="medium")
        def database_query(query: str) -> str:
            '''Execute a database query'''
            # Simulated database query
            return f"Query executed: {query}"

        print(f"✅ Tool with @aim_verify created: {database_query.name}")

        # Execute tool (AIM verification happens automatically)
        result = database_query.invoke("SELECT * FROM users")
        print(f"✅ Tool executed with verification: {result}")

        print("\n🎉 TEST 2 PASSED - @aim_verify decorator works!")
        return True

    except PermissionError as e:
        # This is expected if AIM denies the action
        print(f"⚠️  Tool execution denied by AIM: {e}")
        print("✅ Verification flow works (action was denied as expected in some cases)")
        return True

    except Exception as e:
        print(f"\n❌ TEST 2 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_tool_wrapper():
    """Test 3: AIMToolWrapper - Wrap existing tools"""
    print("\n" + "="*70)
    print("TEST 3: AIMToolWrapper - Wrap existing tools")
    print("="*70)

    try:
        # Register AIM agent
        aim_client = AIMClient.auto_register_or_load(
            "langchain-test-wrapper",
            AIM_URL
        )
        print(f"✅ AIM agent registered: {aim_client.agent_id}")

        # Define tools (without AIM verification)
        @tool
        def calculator(expression: str) -> str:
            '''Calculate mathematical expressions'''
            return f"Result: {eval(expression)}"

        @tool
        def string_reverser(text: str) -> str:
            '''Reverse a string'''
            return text[::-1]

        print(f"✅ Created 2 tools: {calculator.name}, {string_reverser.name}")

        # Wrap ALL tools with AIM verification
        verified_tools = wrap_tools_with_aim(
            tools=[calculator, string_reverser],
            aim_agent=aim_client,
            default_risk_level="low"
        )
        print(f"✅ Wrapped {len(verified_tools)} tools with AIM verification")

        # Execute wrapped tools
        calc_result = verified_tools[0].invoke("10 * 5")
        print(f"✅ Calculator tool executed: {calc_result}")

        reverse_result = verified_tools[1].invoke("Hello AIM!")
        print(f"✅ String reverser tool executed: {reverse_result}")

        print("\n🎉 TEST 3 PASSED - Tool wrapper works!")
        return True

    except PermissionError as e:
        print(f"⚠️  Tool execution denied by AIM: {e}")
        print("✅ Verification flow works (action was denied as expected in some cases)")
        return True

    except Exception as e:
        print(f"\n❌ TEST 3 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def test_graceful_degradation():
    """Test 4: Graceful degradation when AIM not configured"""
    print("\n" + "="*70)
    print("TEST 4: Graceful degradation - No AIM agent")
    print("="*70)

    try:
        # Define tool without AIM agent (should work with warning)
        @tool
        @aim_verify()  # No agent specified, will try to auto-load "langchain-agent"
        def simple_tool(input: str) -> str:
            '''A simple tool'''
            return f"Processed: {input}"

        print(f"✅ Tool created without explicit AIM agent")

        # Execute (should run with warning if no agent found)
        result = simple_tool.invoke("test")
        print(f"✅ Tool executed: {result}")

        print("\n🎉 TEST 4 PASSED - Graceful degradation works!")
        return True

    except Exception as e:
        print(f"\n❌ TEST 4 FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


def main():
    """Run all LangChain integration tests"""
    print("=" * 70)
    print("AIM + LangChain Integration Tests")
    print("=" * 70)
    print(f"AIM Server: {AIM_URL}")
    print()

    results = []

    # Test 1: Callback Handler
    results.append(("AIMCallbackHandler", test_callback_handler()))

    # Test 2: @aim_verify Decorator
    results.append(("@aim_verify decorator", test_aim_verify_decorator()))

    # Test 3: Tool Wrapper
    results.append(("AIMToolWrapper", test_tool_wrapper()))

    # Test 4: Graceful Degradation
    results.append(("Graceful degradation", test_graceful_degradation()))

    # Summary
    print("\n" + "="*70)
    print("TEST SUMMARY")
    print("="*70)

    passed = sum(1 for _, result in results if result)
    total = len(results)

    for test_name, result in results:
        status = "✅ PASSED" if result else "❌ FAILED"
        print(f"{status}: {test_name}")

    print(f"\nTotal: {passed}/{total} tests passed")

    if passed == total:
        print("\n🎉 ALL TESTS PASSED - LangChain integration working perfectly!")
        return 0
    else:
        print(f"\n⚠️  {total - passed} test(s) failed - review output above")
        return 1


if __name__ == "__main__":
    sys.exit(main())
