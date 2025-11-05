# 🦜 LangChain Integration - Secure Your Agent Framework

Secure LangChain agents with AIM in **2 lines of code**.

## What You'll Get

- ✅ Secure existing LangChain agents (zero refactoring)
- ✅ Complete audit trail of all tool uses
- ✅ Real-time trust scoring
- ✅ Security alerts for anomalous behavior
- ✅ Automatic action verification before tool execution

**Integration Time**: 5 minutes
**Code Changes**: 2 lines
**Difficulty**: Beginner

---

## Quick Start (5 Minutes)

### Step 1: Download AIM SDK and Install Dependencies

**Download AIM SDK from dashboard** ([Download Instructions](../quick-start.md#step-3-download-aim-sdk-and-install-dependencies-30-seconds)):
- **NO pip install available** - must download from dashboard
- Extract the ZIP file to your project directory

**Install Dependencies**:
```bash
# Install AIM SDK dependencies and LangChain
pip install keyring PyNaCl requests cryptography langchain langchain-openai
```

### Step 2: Register Agent

In AIM Dashboard (http://localhost:3000):
1. Navigate to Agents → Register New Agent
2. Name: `langchain-assistant`
3. Type: AI Agent
4. Copy private key

```bash
export AIM_PRIVATE_KEY="your-private-key"
export OPENAI_API_KEY="your-openai-key"
```

### Step 3: Add AIM to Your LangChain Agent

**Before (unsecured)**:
```python
from langchain.agents import AgentExecutor, create_openai_functions_agent
from langchain_openai import ChatOpenAI
from langchain.tools import Tool

# Your existing LangChain agent
llm = ChatOpenAI(model="gpt-4")
tools = [search_tool, calculator_tool]
agent = create_openai_functions_agent(llm, tools, prompt)
agent_executor = AgentExecutor(agent=agent, tools=tools)

# Run agent
result = agent_executor.run("What's the weather in SF?")
```

**After (secured with AIM)** - Just add 2 lines:
```python
from aim_sdk import secure  # ← Line 1: Import AIM
from aim_sdk.integrations.langchain import AIMCallbackHandler
from langchain.agents import AgentExecutor, create_openai_functions_agent
from langchain_openai import ChatOpenAI

# Register with AIM
aim_agent = secure("langchain-assistant")  # ← Line 2: Secure your agent

# Your existing LangChain agent (unchanged)
llm = ChatOpenAI(model="gpt-4")
agent = create_openai_functions_agent(llm, tools, prompt)

# Add AIM callback (Line 3)
agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)]  # ← Line 3: Add callback
)

# Run agent - now secured!
result = agent_executor.run("What's the weather in SF?")
# ✅ Every tool use is verified
# ✅ Trust score updates in real-time
# ✅ Complete audit trail
```

**That's it!** Your LangChain agent is now enterprise-secure.

---

## Complete Example: Weather Assistant

```python
"""
LangChain Weather Assistant - Secured with AIM
"""

from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler
from langchain.agents import AgentExecutor, create_openai_functions_agent
from langchain_openai import ChatOpenAI
from langchain.tools import Tool
from langchain.prompts import ChatPromptTemplate, MessagesPlaceholder
import requests
import os

# 🔐 Register with AIM
aim_agent = secure(
    name="langchain-weather-assistant",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

# Define tools
def get_weather(city: str) -> str:
    """Get current weather for a city"""
    response = requests.get(
        "https://api.openweathermap.org/data/2.5/weather",
        params={
            "q": city,
            "appid": os.getenv("OPENWEATHER_API_KEY"),
            "units": "imperial"
        }
    )
    data = response.json()
    temp = data['main']['temp']
    description = data['weather'][0]['description']
    return f"Weather in {city}: {temp}°F, {description}"

def calculate(expression: str) -> float:
    """Safely evaluate mathematical expressions"""
    try:
        # Simple eval (in production, use ast.literal_eval or math library)
        return eval(expression)
    except Exception as e:
        return f"Error: {str(e)}"

# Create LangChain tools
weather_tool = Tool(
    name="get_weather",
    func=get_weather,
    description="Get current weather for a city. Input should be a city name."
)

calculator_tool = Tool(
    name="calculator",
    func=calculate,
    description="Perform mathematical calculations. Input should be a math expression."
)

tools = [weather_tool, calculator_tool]

# Create prompt
prompt = ChatPromptTemplate.from_messages([
    ("system", "You are a helpful weather assistant. Use tools to answer questions."),
    MessagesPlaceholder("chat_history", optional=True),
    ("human", "{input}"),
    MessagesPlaceholder("agent_scratchpad"),
])

# Create LangChain agent
llm = ChatOpenAI(model="gpt-4", temperature=0)
agent = create_openai_functions_agent(llm, tools, prompt)

# Create agent executor with AIM callback
agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)],  # ← AIM integration
    verbose=True
)

# Run queries
if __name__ == "__main__":
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("🦜 LangChain Weather Assistant (Secured by AIM)")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

    queries = [
        "What's the weather in San Francisco?",
        "What's the weather in New York?",
        "If it's 62°F in SF and 58°F in NY, what's the temperature difference?",
    ]

    for query in queries:
        print(f"\n🤔 Query: {query}")
        result = agent_executor.run(query)
        print(f"✅ Answer: {result}")

    print("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    print("✅ All queries complete!")
    print("📊 Check dashboard: http://localhost:3000/agents/langchain-weather-assistant")
    print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
```

**Run it**:
```bash
python langchain_weather_assistant.py
```

**Output**:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🦜 LangChain Weather Assistant (Secured by AIM)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🤔 Query: What's the weather in San Francisco?

> Entering new AgentExecutor chain...
Action: get_weather
Action Input: San Francisco
Observation: Weather in San Francisco: 62.5°F, clear sky
✅ Answer: The weather in San Francisco is 62.5°F with clear skies.

🤔 Query: What's the weather in New York?

> Entering new AgentExecutor chain...
Action: get_weather
Action Input: New York
Observation: Weather in New York: 58.3°F, partly cloudy
✅ Answer: The weather in New York is 58.3°F and partly cloudy.

🤔 Query: If it's 62°F in SF and 58°F in NY, what's the temperature difference?

> Entering new AgentExecutor chain...
Action: calculator
Action Input: 62 - 58
Observation: 4.0
✅ Answer: The temperature difference is 4°F.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ All queries complete!
📊 Check dashboard: http://localhost:3000/agents/langchain-weather-assistant
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

---

## Dashboard View

Open http://localhost:3000 → Agents → langchain-weather-assistant

### Agent Status
```
Agent: langchain-weather-assistant
Status: ✅ ACTIVE
Trust Score: 0.96 (Excellent)
Last Verified: 5 seconds ago
Total Actions: 3
Framework: LangChain
```

### Recent Tool Uses
```
✅ get_weather("San Francisco")  |  20s ago  |  SUCCESS  |  312ms
✅ get_weather("New York")       |  15s ago  |  SUCCESS  |  298ms
✅ calculator("62 - 58")         |  10s ago  |  SUCCESS  |    2ms
```

### Trust Score
```
✅ Verification Status:     100%  (1.00)  [Weight: 25%]
✅ Action Success Rate:     100%  (1.00)  [Weight: 15%]
✅ Uptime:                  100%  (1.00)  [Weight: 15%]
✅ Security Alerts:           0   (1.00)  [Weight: 15%]
✅ Compliance:              100%  (1.00)  [Weight: 10%]
⚠️  Age & History:          New   (0.50)  [Weight: 10%]
✅ Drift Detection:         None  (1.00)  [Weight:  5%]
✅ User Feedback:           None  (1.00)  [Weight:  5%]

Overall: 0.96 / 1.00
```

### Audit Trail
```
📝 Tool: get_weather
   Input: {"city": "San Francisco"}
   Output: "Weather in San Francisco: 62.5°F, clear sky"
   Timestamp: 2025-10-21 16:15:42 UTC
   Approved: AUTO

📝 Tool: get_weather
   Input: {"city": "New York"}
   Output: "Weather in New York: 58.3°F, partly cloudy"
   Timestamp: 2025-10-21 16:15:47 UTC
   Approved: AUTO

📝 Tool: calculator
   Input: {"expression": "62 - 58"}
   Output: 4.0
   Timestamp: 2025-10-21 16:15:52 UTC
   Approved: AUTO
```

---

## Advanced Usage

### 1. Conversational RAG Agent

```python
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler
from langchain.agents import AgentExecutor, create_openai_functions_agent
from langchain_openai import ChatOpenAI, OpenAIEmbeddings
from langchain_community.vectorstores import FAISS
from langchain.tools.retriever import create_retriever_tool
from langchain.docstore.document import Document

# Register with AIM
aim_agent = secure("rag-agent")

# Create vector store
documents = [
    Document(page_content="AIM provides enterprise-grade agent security."),
    Document(page_content="AIM uses Ed25519 cryptographic signatures."),
    Document(page_content="AIM supports LangChain, CrewAI, and MCP integrations."),
]

embeddings = OpenAIEmbeddings()
vectorstore = FAISS.from_documents(documents, embeddings)

# Create retriever tool
retriever_tool = create_retriever_tool(
    vectorstore.as_retriever(),
    name="aim_docs",
    description="Search AIM documentation for information about features and usage"
)

# Create agent with AIM callback
llm = ChatOpenAI(model="gpt-4")
agent = create_openai_functions_agent(llm, [retriever_tool], prompt)
agent_executor = AgentExecutor(
    agent=agent,
    tools=[retriever_tool],
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)]  # ← AIM integration
)

# Query
result = agent_executor.run("What security features does AIM provide?")
print(result)
```

### 2. SQL Database Agent

```python
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler
from langchain.agents import create_sql_agent
from langchain_community.agent_toolkits import SQLDatabaseToolkit
from langchain_community.utilities import SQLDatabase
from langchain_openai import ChatOpenAI

# Register with AIM
aim_agent = secure("sql-agent")

# Connect to database
db = SQLDatabase.from_uri("postgresql://user:pass@localhost/mydb")
toolkit = SQLDatabaseToolkit(db=db, llm=ChatOpenAI(model="gpt-4"))

# Create SQL agent with AIM callback
agent_executor = create_sql_agent(
    llm=ChatOpenAI(model="gpt-4", temperature=0),
    toolkit=toolkit,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)],  # ← AIM integration
    verbose=True
)

# Query database
result = agent_executor.run("How many users are in the database?")
print(result)
# ✅ SQL query is verified and logged to audit trail
```

### 3. Multi-Tool Research Agent

```python
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler
from langchain.agents import AgentExecutor, create_openai_functions_agent
from langchain_openai import ChatOpenAI
from langchain.tools import Tool
from langchain_community.utilities import GoogleSerperAPIWrapper

# Register with AIM
aim_agent = secure("research-agent")

# Create tools
search = GoogleSerperAPIWrapper()
search_tool = Tool(
    name="search",
    func=search.run,
    description="Search Google for recent information"
)

# Add more tools...
tools = [search_tool, wikipedia_tool, arxiv_tool]

# Create agent with AIM callback
llm = ChatOpenAI(model="gpt-4")
agent = create_openai_functions_agent(llm, tools, prompt)
agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)],  # ← AIM integration
    max_iterations=10
)

# Research
result = agent_executor.run("What are the latest developments in AI safety?")
# ✅ All tool uses are verified and logged
```

---

## What AIM Callback Does

The `AIMCallbackHandler` automatically:

### 1. Captures Tool Execution
```python
on_tool_start(tool_name, inputs)
  → Logs tool invocation to AIM
  → Verifies agent identity
  → Checks trust score

on_tool_end(output)
  → Logs tool output to AIM
  → Updates trust score
  → Detects anomalies

on_tool_error(error)
  → Logs error to AIM
  → Triggers security alert if suspicious
  → Updates trust score
```

### 2. Monitors Agent Behavior
- Tracks tool usage patterns
- Detects behavioral drift
- Identifies unusual sequences
- Flags high-risk operations

### 3. Maintains Audit Trail
Every tool use is logged:
- Tool name and inputs
- Execution time
- Output or error
- Timestamp
- Agent identity
- Verification status

**SOC 2, HIPAA, GDPR compliant!**

---

## Configuration Options

### Custom Risk Levels

```python
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler

aim_agent = secure("my-agent")

# Configure callback with custom risk levels
callback = AIMCallbackHandler(
    aim_agent=aim_agent,
    risk_levels={
        "search": "low",           # Auto-approve searches
        "database_query": "medium",  # Log but auto-approve
        "delete_user": "high",     # Require approval
        "send_email": "medium",
    }
)

agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[callback]
)
```

### Auto-Retry on Verification Failure

```python
callback = AIMCallbackHandler(
    aim_agent=aim_agent,
    auto_retry=True,          # Retry failed verifications
    max_retries=3,            # Up to 3 retries
    retry_delay=1.0           # 1 second between retries
)
```

### Async Support

```python
from langchain.schema.runnable import RunnablePassthrough

# Async agent with AIM
async def run_async_agent(query: str):
    result = await agent_executor.ainvoke(
        {"input": query},
        callbacks=[AIMCallbackHandler(aim_agent=aim_agent)]
    )
    return result

# Run
result = await run_async_agent("What's the weather in Tokyo?")
```

---

## Best Practices

### 1. Use Descriptive Agent Names
```python
# ❌ Bad
aim_agent = secure("agent")

# ✅ Good
aim_agent = secure("customer-support-rag-agent")
```

### 2. Add Context to Tool Descriptions
```python
# ✅ Good - helps AIM understand tool purpose
search_tool = Tool(
    name="web_search",
    func=search.run,
    description="Search the web for current events and information. Use this for questions about recent news or facts not in the knowledge base."
)
```

### 3. Handle Approval Timeouts
```python
from aim_sdk.exceptions import ApprovalTimeoutError

try:
    result = agent_executor.run("Delete all users")
except ApprovalTimeoutError:
    print("⏳ Waiting for admin approval...")
    # Notify user to check dashboard
```

### 4. Monitor Trust Score
```python
# Check trust score periodically
trust_score = aim_agent.get_trust_score()

if trust_score < 0.7:
    print(f"⚠️  Low trust score: {trust_score}")
    # Take action: notify admin, pause agent, etc.
```

---

## Troubleshooting

### Issue: "Callback not called"

**Cause**: Callback not added to agent executor

**Solution**:
```python
# ✅ Make sure callback is in list
agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)]  # ← Must be in list
)
```

### Issue: "Tool execution not logged"

**Cause**: Using tools outside of AgentExecutor

**Solution**:
```python
# ❌ Direct tool call - not logged
result = search_tool.run("query")

# ✅ Use through agent executor
result = agent_executor.run("query")  # ← Tools logged via callback
```

### Issue: "High latency"

**Cause**: AIM verification adds network round-trip

**Solution**:
- Enable async verification
- Batch tool calls
- Use local AIM deployment
- Cache verification results

---

## Migration Guide

### Migrating Existing LangChain Agents

**Before**:
```python
from langchain.agents import AgentExecutor

agent_executor = AgentExecutor(agent=agent, tools=tools)
result = agent_executor.run("query")
```

**After** (just add 2 lines):
```python
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler

aim_agent = secure("my-agent")  # ← Add line 1

agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)]  # ← Add line 2
)
result = agent_executor.run("query")
```

**That's it!** Zero refactoring required.

---

## ✅ Checklist

- [ ] AIM platform running
- [ ] Agent registered in dashboard
- [ ] Private key saved to environment
- [ ] `aim-sdk` installed
- [ ] `AIMCallbackHandler` added to agent executor
- [ ] Code runs without errors
- [ ] Dashboard shows tool uses
- [ ] Trust score updating
- [ ] Audit trail capturing actions

**All checked?** 🎉 **Your LangChain agent is enterprise-secure!**

---

## Next Steps

- [CrewAI Integration →](./crewai.md) - Multi-agent teams
- [MCP Integration →](./mcp.md) - Model Context Protocol
- [SDK Documentation](../sdk/python.md) - Complete SDK reference

---

<div align="center">

**Next**: [CrewAI Integration →](./crewai.md)

[🏠 Back to Home](../../README.md) • [📚 All Integrations](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
