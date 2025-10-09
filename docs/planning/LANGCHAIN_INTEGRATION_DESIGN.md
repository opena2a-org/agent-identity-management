# 🦜 AIM + LangChain Integration Design

**Date**: October 8, 2025
**Status**: 📋 Design Phase
**Goal**: Seamless AIM verification for all LangChain tools and chains

---

## 🎯 Objectives

### Primary Goal
**Enable automatic AIM verification for LangChain tools** with minimal code changes.

### Success Criteria
1. ✅ LangChain developers can verify tools with a single decorator
2. ✅ All tool invocations automatically logged to AIM
3. ✅ Zero breaking changes to existing LangChain code
4. ✅ Support for both simple tools and complex chains
5. ✅ Production-ready error handling and logging

---

## 🏗️ Architecture Overview

### Integration Points

```
┌─────────────────────────────────────────────────────────┐
│                    LangChain Application                │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐         ┌──────────────┐            │
│  │ LLM/Chat     │         │ Tools        │            │
│  │ Models       │         │ (@tool)      │            │
│  └──────┬───────┘         └──────┬───────┘            │
│         │                        │                     │
│         │  ┌─────────────────────┴─────────┐          │
│         │  │  AIM Integration Layer        │          │
│         │  ├───────────────────────────────┤          │
│         │  │                               │          │
│         └──┤  1. AIMCallbackHandler        │          │
│            │     - Logs all tool calls     │          │
│            │     - Tracks actions          │          │
│            │     - Reports to AIM API      │          │
│            │                               │          │
│            │  2. @aim_verify Decorator     │          │
│            │     - Wraps LangChain tools   │          │
│            │     - Verifies before exec    │          │
│            │     - Logs to AIM             │          │
│            │                               │          │
│            │  3. AIMToolWrapper            │          │
│            │     - Converts LC tools       │          │
│            │     - Adds AIM verification   │          │
│            │     - Maintains LC interface  │          │
│            └───────────────────────────────┘          │
│                         │                              │
└─────────────────────────┼──────────────────────────────┘
                          │
                          ▼
                ┌─────────────────┐
                │  AIM Backend    │
                │  - Agent verify │
                │  - Action log   │
                │  - Trust score  │
                └─────────────────┘
```

---

## 📦 Components to Implement

### 1. AIMCallbackHandler (Automatic Logging)

**Purpose**: Automatically log all LangChain tool invocations to AIM

**File**: `sdks/python/aim_sdk/integrations/langchain/callback.py`

**Implementation**:
```python
from langchain_core.callbacks import BaseCallbackHandler
from typing import Any, Dict, List, Optional
from aim_sdk import AIMClient

class AIMCallbackHandler(BaseCallbackHandler):
    """
    LangChain callback handler that logs all tool calls to AIM.

    Usage:
        from aim_sdk.integrations.langchain import AIMCallbackHandler

        aim_handler = AIMCallbackHandler(
            agent=aim_client,
            log_inputs=True,
            log_outputs=True
        )

        # Attach to LangChain chain/agent
        agent = create_react_agent(
            llm=ChatOpenAI(),
            tools=tools,
            callbacks=[aim_handler]
        )
    """

    def __init__(
        self,
        agent: AIMClient,
        log_inputs: bool = True,
        log_outputs: bool = True,
        log_errors: bool = True
    ):
        super().__init__()
        self.agent = agent
        self.log_inputs = log_inputs
        self.log_outputs = log_outputs
        self.log_errors = log_errors
        self._active_tools: Dict[str, Dict[str, Any]] = {}

    def on_tool_start(
        self,
        serialized: Dict[str, Any],
        input_str: str,
        *,
        run_id: str,
        parent_run_id: Optional[str] = None,
        tags: Optional[List[str]] = None,
        metadata: Optional[Dict[str, Any]] = None,
        **kwargs: Any
    ) -> Any:
        """Log when tool execution starts"""
        tool_name = serialized.get("name", "unknown_tool")

        # Store tool invocation details
        self._active_tools[run_id] = {
            "tool_name": tool_name,
            "input": input_str if self.log_inputs else None,
            "tags": tags or [],
            "metadata": metadata or {}
        }

        # Log to AIM (action start)
        try:
            # This will be implemented after we add action logging endpoint
            pass
        except Exception as e:
            if self.log_errors:
                print(f"AIM logging error: {e}")

    def on_tool_end(
        self,
        output: str,
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Log when tool execution completes"""
        if run_id not in self._active_tools:
            return

        tool_data = self._active_tools.pop(run_id)
        tool_name = tool_data["tool_name"]

        # Log successful tool execution to AIM
        try:
            self.agent.perform_action_sync(
                action_name=f"langchain_tool:{tool_name}",
                resource=tool_data.get("input", "")[:100],  # First 100 chars
                metadata={
                    "output": output[:500] if self.log_outputs else None,
                    "tags": tool_data.get("tags", []),
                    **tool_data.get("metadata", {})
                },
                risk_level="low"
            )
        except Exception as e:
            if self.log_errors:
                print(f"AIM logging error: {e}")

    def on_tool_error(
        self,
        error: BaseException,
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Log when tool execution fails"""
        if run_id not in self._active_tools:
            return

        tool_data = self._active_tools.pop(run_id)

        # Log error to AIM
        if self.log_errors:
            try:
                self.agent.perform_action_sync(
                    action_name=f"langchain_tool:{tool_data['tool_name']}",
                    resource=tool_data.get("input", "")[:100],
                    metadata={
                        "error": str(error),
                        "status": "failed"
                    },
                    risk_level="medium"
                )
            except Exception as e:
                print(f"AIM logging error: {e}")
```

**Benefits**:
- ✅ Zero code changes to existing tools
- ✅ Automatic logging of all tool calls
- ✅ Tracks success and failures
- ✅ Minimal performance overhead

---

### 2. @aim_verify Decorator (Explicit Verification)

**Purpose**: Decorator to verify tool execution before running

**File**: `sdks/python/aim_sdk/integrations/langchain/decorators.py`

**Implementation**:
```python
from functools import wraps
from typing import Callable, Optional, Any
from aim_sdk import AIMClient

def aim_verify(
    agent: Optional[AIMClient] = None,
    action_name: Optional[str] = None,
    risk_level: str = "medium",
    resource: Optional[str] = None
):
    """
    Decorator to add AIM verification to LangChain tools.

    Usage:
        from aim_sdk.integrations.langchain import aim_verify
        from langchain_core.tools import tool

        # Option 1: Use with explicit agent
        @tool
        @aim_verify(agent=my_aim_client, risk_level="high")
        def delete_user(user_id: str) -> str:
            '''Delete a user from the database'''
            return f"Deleted user {user_id}"

        # Option 2: Use with auto-loaded agent
        @tool
        @aim_verify(action_name="database_query")
        def query_database(query: str) -> str:
            '''Execute a database query'''
            return execute_query(query)
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            # Auto-load agent if not provided
            _agent = agent
            if _agent is None:
                # Try to load from environment or default credentials
                try:
                    _agent = AIMClient.from_credentials("langchain-agent")
                except FileNotFoundError:
                    # No AIM agent configured - run without verification
                    print(f"Warning: No AIM agent configured for {func.__name__}")
                    return func(*args, **kwargs)

            # Determine action name
            _action_name = action_name or f"langchain_tool:{func.__name__}"

            # Determine resource
            _resource = resource
            if _resource is None and args:
                _resource = str(args[0])[:100]

            # Verify with AIM before execution
            try:
                _agent.perform_action_sync(
                    action_name=_action_name,
                    resource=_resource or "",
                    metadata={
                        "function": func.__name__,
                        "args": str(args)[:200],
                        "kwargs": str(kwargs)[:200]
                    },
                    risk_level=risk_level
                )
            except Exception as e:
                # Verification failed - raise error
                raise PermissionError(f"AIM verification failed: {e}")

            # Execute the actual function
            result = func(*args, **kwargs)

            # Log successful completion
            try:
                _agent.perform_action_sync(
                    action_name=f"{_action_name}_complete",
                    resource=_resource or "",
                    metadata={"status": "success"},
                    risk_level="low"
                )
            except Exception:
                pass  # Don't fail on logging errors

            return result

        return wrapper
    return decorator
```

**Benefits**:
- ✅ Explicit verification control
- ✅ Works with `@tool` decorator
- ✅ Graceful degradation (runs without AIM if not configured)
- ✅ Clear error messages

---

### 3. AIMToolWrapper (Wrap Existing Tools)

**Purpose**: Wrap existing LangChain tools with AIM verification

**File**: `sdks/python/aim_sdk/integrations/langchain/tools.py`

**Implementation**:
```python
from langchain_core.tools import BaseTool, StructuredTool
from typing import Any, Optional, Type, Callable
from pydantic import BaseModel
from aim_sdk import AIMClient

class AIMToolWrapper(BaseTool):
    """
    Wraps a LangChain tool with AIM verification.

    Usage:
        from langchain_core.tools import tool
        from aim_sdk.integrations.langchain import AIMToolWrapper

        # Original LangChain tool
        @tool
        def my_tool(input: str) -> str:
            '''My tool description'''
            return f"Processed: {input}"

        # Wrap with AIM verification
        verified_tool = AIMToolWrapper(
            tool=my_tool,
            aim_agent=my_aim_client,
            risk_level="high"
        )

        # Use in LangChain as normal
        tools = [verified_tool]
    """

    name: str
    description: str
    aim_agent: AIMClient
    wrapped_tool: BaseTool
    risk_level: str = "medium"

    def _run(self, *args, **kwargs) -> Any:
        """Execute tool with AIM verification"""
        # Verify with AIM
        try:
            self.aim_agent.perform_action_sync(
                action_name=f"langchain_tool:{self.name}",
                resource=str(args[0]) if args else "",
                metadata={"tool": self.name},
                risk_level=self.risk_level
            )
        except Exception as e:
            raise PermissionError(f"AIM verification failed: {e}")

        # Execute wrapped tool
        return self.wrapped_tool._run(*args, **kwargs)

    async def _arun(self, *args, **kwargs) -> Any:
        """Async execution with AIM verification"""
        # Same verification logic
        try:
            self.aim_agent.perform_action_sync(
                action_name=f"langchain_tool:{self.name}",
                resource=str(args[0]) if args else "",
                metadata={"tool": self.name},
                risk_level=self.risk_level
            )
        except Exception as e:
            raise PermissionError(f"AIM verification failed: {e}")

        # Execute wrapped tool asynchronously
        return await self.wrapped_tool._arun(*args, **kwargs)


def wrap_tools_with_aim(
    tools: list[BaseTool],
    aim_agent: AIMClient,
    default_risk_level: str = "medium"
) -> list[BaseTool]:
    """
    Convenience function to wrap multiple tools at once.

    Usage:
        tools = [tool1, tool2, tool3]
        verified_tools = wrap_tools_with_aim(tools, my_aim_client)
    """
    return [
        AIMToolWrapper(
            name=tool.name,
            description=tool.description,
            aim_agent=aim_agent,
            wrapped_tool=tool,
            risk_level=default_risk_level
        )
        for tool in tools
    ]
```

---

## 📚 Usage Examples

### Example 1: Automatic Logging (Simplest)

```python
from langchain_openai import ChatOpenAI
from langchain_core.tools import tool
from langchain.agents import create_react_agent
from aim_sdk import AIMClient
from aim_sdk.integrations.langchain import AIMCallbackHandler

# Register AIM agent
aim_client = AIMClient.auto_register_or_load("langchain-agent", "https://aim.company.com")

# Create callback handler
aim_handler = AIMCallbackHandler(agent=aim_client)

# Define tools (normal LangChain code)
@tool
def search_database(query: str) -> str:
    '''Search the database'''
    return f"Results for {query}"

# Create agent with AIM logging
agent = create_react_agent(
    llm=ChatOpenAI(),
    tools=[search_database],
    callbacks=[aim_handler]  # ← Only change needed!
)

# All tool calls automatically logged to AIM
agent.run("Find user john@example.com")
```

### Example 2: Explicit Verification (Most Secure)

```python
from langchain_core.tools import tool
from aim_sdk.integrations.langchain import aim_verify
from aim_sdk import AIMClient

# Register AIM agent
aim_client = AIMClient.auto_register_or_load("langchain-agent", "https://aim.company.com")

# High-risk tool with verification
@tool
@aim_verify(agent=aim_client, risk_level="high")
def delete_user(user_id: str) -> str:
    '''Delete a user from the database'''
    # AIM verification happens before this code runs
    return f"Deleted user {user_id}"

# Medium-risk tool
@tool
@aim_verify(agent=aim_client, risk_level="medium")
def update_email(user_id: str, email: str) -> str:
    '''Update user email'''
    return f"Updated {user_id} email to {email}"

# Low-risk tool
@tool
@aim_verify(agent=aim_client, risk_level="low")
def read_profile(user_id: str) -> str:
    '''Read user profile'''
    return f"Profile for {user_id}"
```

### Example 3: Wrap Existing Tools

```python
from langchain_community.tools import WikipediaQueryRun
from langchain_core.tools import tool
from aim_sdk.integrations.langchain import wrap_tools_with_aim
from aim_sdk import AIMClient

# Register AIM agent
aim_client = AIMClient.auto_register_or_load("langchain-agent", "https://aim.company.com")

# Existing LangChain tools (no modification)
@tool
def calculator(expression: str) -> str:
    '''Calculate mathematical expressions'''
    return str(eval(expression))

wikipedia = WikipediaQueryRun()

# Wrap ALL tools with AIM verification
verified_tools = wrap_tools_with_aim(
    tools=[calculator, wikipedia],
    aim_agent=aim_client,
    default_risk_level="medium"
)

# Use in LangChain as normal
agent = create_react_agent(
    llm=ChatOpenAI(),
    tools=verified_tools  # All tools now AIM-verified!
)
```

---

## 🔄 Implementation Plan

### Phase 1: Core Components (4 hours)
- [ ] Create `sdks/python/aim_sdk/integrations/langchain/` directory
- [ ] Implement `AIMCallbackHandler` (2 hours)
- [ ] Implement `@aim_verify` decorator (1.5 hours)
- [ ] Implement `AIMToolWrapper` (30 minutes)

### Phase 2: Testing (2 hours)
- [ ] Install LangChain dependencies
- [ ] Create integration tests
- [ ] Test with sample LangChain agents
- [ ] Verify AIM logging works correctly

### Phase 3: Documentation (2 hours)
- [ ] Write integration guide
- [ ] Create example scripts
- [ ] Update main README
- [ ] Add troubleshooting section

**Total Estimated Time**: 8 hours

---

## 🎯 Success Metrics

### Functional Requirements
- ✅ Tools can be verified with `@aim_verify` decorator
- ✅ All tool calls logged via `AIMCallbackHandler`
- ✅ Existing tools can be wrapped with `AIMToolWrapper`
- ✅ Zero breaking changes to LangChain code
- ✅ Graceful degradation if AIM not configured

### Non-Functional Requirements
- ✅ Performance overhead < 50ms per tool call
- ✅ Error messages are clear and actionable
- ✅ Documentation with 3+ working examples
- ✅ Compatible with LangChain 0.1.0+

---

## 🚀 Next Steps

1. Create integration directory structure
2. Implement `AIMCallbackHandler` first (most valuable)
3. Add comprehensive tests
4. Document with examples
5. Release as `aim_sdk.integrations.langchain`

---

**Design Date**: October 8, 2025
**Status**: ✅ Design Complete - Ready for Implementation
**Estimated Implementation Time**: 8 hours

---

**END OF DESIGN DOCUMENT**
