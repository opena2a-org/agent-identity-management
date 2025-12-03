# 🤝 CrewAI Integration - Secure Multi-Agent Teams

Secure your entire CrewAI crew with **3 lines of code**.

## What You'll Build

A multi-agent CrewAI team that:
- ✅ Multiple specialized agents working together
- ✅ Automatically secured with AIM (3 lines of code)
- ✅ Complete audit trail of all agent actions
- ✅ Real-time trust scoring for each agent
- ✅ Team-level security monitoring

**Integration Time**: 5 minutes
**Code Changes**: 3 lines
**Use Case**: Multi-agent workflows (research teams, content creation, business intelligence)

---

## Prerequisites

1. ✅ AIM platform running ([Quick Start Guide](../quick-start.md))
2. ✅ CrewAI installed (`pip install crewai crewai-tools`)
3. ✅ AIM SDK downloaded from dashboard ([Download Instructions](../quick-start.md#step-3-download-aim-sdk-and-install-dependencies-30-seconds))
   - **NO pip install available** - must download from dashboard
   - Dependencies: `pip install keyring PyNaCl requests cryptography`
4. ✅ OpenAI API key (for LLM)

---

## Integration Pattern

### Before (Unsecured CrewAI)

```python
from crewai import Agent, Task, Crew

# Define agents
researcher = Agent(
    role="Researcher",
    goal="Research and gather information",
    tools=[search_tool, scrape_tool]
)

writer = Agent(
    role="Writer",
    goal="Write engaging content",
    tools=[write_tool]
)

# Create crew
crew = Crew(agents=[researcher, writer], tasks=[research_task, write_task])

# Run crew (no security, no audit trail)
result = crew.kickoff()
```

### After (Secured with AIM) - Just 3 Lines

```python
from crewai import Agent, Task, Crew
from aim_sdk import secure  # ← Line 1
from aim_sdk.integrations.crewai import AIMCrewWrapper  # ← Line 2

# Register crew with AIM
aim_crew = secure("research-crew")  # ← Line 3

# Define agents (same as before)
researcher = Agent(
    role="Researcher",
    goal="Research and gather information",
    tools=[search_tool, scrape_tool]
)

writer = Agent(
    role="Writer",
    goal="Write engaging content",
    tools=[write_tool]
)

# Create crew
crew = Crew(agents=[researcher, writer], tasks=[research_task, write_task])

# Wrap crew with AIM security
secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# Run crew (fully secured and monitored)
result = secured_crew.kickoff()
# ✅ Every agent action verified and logged
```

**What Changes?**
- **Before**: No security, no audit trail, no visibility
- **After**: Complete visibility, audit trail, trust scoring, security alerts

---

## Step 1: Register Crew (30 seconds)

### In AIM Dashboard

1. **Login** to http://localhost:3000
2. **Navigate**: Agents → Register New Agent
3. **Fill in**:
   ```
   Agent Name: research-crew
   Agent Type: Multi-Agent Team
   Description: CrewAI research team (Researcher + Writer)
   ```
4. **Click** "Register Agent"
5. **Copy** the private key

### Save Private Key

```bash
# Save to environment variable
export AIM_PRIVATE_KEY="your-private-key-here"
export OPENAI_API_KEY="your-openai-api-key"
```

---

## Step 2: Complete Example - Research Crew (5 minutes)

Create `research_crew.py`:

```python
"""
Research Crew - Secured with AIM
Multi-agent team: Researcher + Writer + Editor
"""

from aim_sdk import secure
from aim_sdk.integrations.crewai import AIMCrewWrapper
from crewai import Agent, Task, Crew, Process
from crewai_tools import SerperDevTool, ScrapeWebsiteTool
from langchain_openai import ChatOpenAI
import os

# 🔐 ONE LINE - Secure your crew!
aim_crew = secure(
    name="research-crew",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

# Initialize LLM
llm = ChatOpenAI(model="gpt-4", temperature=0.7)

# Initialize tools
search_tool = SerperDevTool()
scrape_tool = ScrapeWebsiteTool()

# ═══════════════════════════════════════════════════════════
# DEFINE AGENTS
# ═══════════════════════════════════════════════════════════

researcher = Agent(
    role="Senior Research Analyst",
    goal="Uncover cutting-edge developments in {topic}",
    backstory="""You are a seasoned research analyst with a knack for
    uncovering the latest developments in technology. Known for your ability
    to find the most relevant information and present it clearly.""",
    verbose=True,
    allow_delegation=False,
    tools=[search_tool, scrape_tool],
    llm=llm
)

writer = Agent(
    role="Tech Content Writer",
    goal="Write engaging and accessible articles on {topic}",
    backstory="""You are a renowned technical writer, known for your ability
    to translate complex technical concepts into clear, engaging content.
    You have a talent for storytelling and making technology accessible.""",
    verbose=True,
    allow_delegation=False,
    llm=llm
)

editor = Agent(
    role="Content Editor",
    goal="Review and polish the final article on {topic}",
    backstory="""You are a meticulous editor with an eye for detail. You ensure
    every piece of content is polished, grammatically correct, and engaging.""",
    verbose=True,
    allow_delegation=False,
    llm=llm
)

# ═══════════════════════════════════════════════════════════
# DEFINE TASKS
# ═══════════════════════════════════════════════════════════

research_task = Task(
    description="""Conduct comprehensive research on {topic}.

    Your research should:
    1. Identify the latest developments and trends
    2. Find key statistics and data points
    3. Discover expert opinions and analysis
    4. Locate credible sources and references

    Make sure to use the search and scrape tools to gather information.""",
    expected_output="A detailed research report with sources and key findings",
    agent=researcher
)

writing_task = Task(
    description="""Based on the research, write a compelling 800-word article on {topic}.

    Your article should:
    1. Have an engaging introduction
    2. Present key findings clearly
    3. Include relevant statistics and quotes
    4. End with actionable insights
    5. Be accessible to a general tech-savvy audience

    Make the content engaging and well-structured.""",
    expected_output="An 800-word article draft on {topic}",
    agent=writer,
    context=[research_task]
)

editing_task = Task(
    description="""Review and polish the article on {topic}.

    Your review should:
    1. Fix any grammatical errors
    2. Improve clarity and flow
    3. Ensure consistent tone and style
    4. Verify all facts are accurate
    5. Optimize for readability

    Return the final polished version.""",
    expected_output="A polished, publication-ready article on {topic}",
    agent=editor,
    context=[writing_task]
)

# ═══════════════════════════════════════════════════════════
# CREATE CREW
# ═══════════════════════════════════════════════════════════

crew = Crew(
    agents=[researcher, writer, editor],
    tasks=[research_task, writing_task, editing_task],
    process=Process.sequential,  # Tasks run in sequence
    verbose=2
)

# 🔐 SECURE THE CREW WITH AIM
secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# ═══════════════════════════════════════════════════════════
# RUN THE CREW
# ═══════════════════════════════════════════════════════════

def research_and_write(topic: str) -> str:
    """
    Research a topic and write an article about it

    Args:
        topic: The topic to research and write about

    Returns:
        The final polished article

    Example:
        >>> article = research_and_write("Artificial Intelligence in Healthcare")
        >>> print(article)
    """
    result = secured_crew.kickoff(inputs={"topic": topic})
    return result


def main():
    """Demo the research crew"""
    print("🚀 Starting Research Crew...")
    print()

    # Example: Research AI in Healthcare
    topic = "Artificial Intelligence in Healthcare 2025"

    print(f"📚 Researching: {topic}")
    print()

    article = research_and_write(topic)

    print()
    print("=" * 80)
    print("✅ FINAL ARTICLE")
    print("=" * 80)
    print()
    print(article)
    print()
    print("=" * 80)
    print()
    print("✅ Done! Check your AIM dashboard for the audit trail.")


if __name__ == "__main__":
    main()
```

---

## Step 3: Run Your Secured Crew

```bash
# Make sure environment variables are set
export AIM_PRIVATE_KEY="your-key"
export OPENAI_API_KEY="your-openai-key"
export SERPER_API_KEY="your-serper-key"  # For search tool
export AIM_URL="http://localhost:8080"

# Run the crew
python research_crew.py
```

**Expected Output**:
```
🚀 Starting Research Crew...

📚 Researching: Artificial Intelligence in Healthcare 2025

[Researcher] Starting task: Conduct comprehensive research...
[Researcher] Using search_tool to find latest AI healthcare developments...
[Researcher] Found 15 relevant articles...
[Researcher] Task complete!

[Writer] Starting task: Write compelling article...
[Writer] Drafting introduction...
[Writer] Writing key findings section...
[Writer] Adding statistics and quotes...
[Writer] Task complete!

[Editor] Starting task: Review and polish article...
[Editor] Checking grammar and flow...
[Editor] Optimizing readability...
[Editor] Task complete!

================================================================================
✅ FINAL ARTICLE
================================================================================

AI in Healthcare 2025: The Revolution is Here

The healthcare industry stands at the precipice of a transformation...
[800 words of polished content]

================================================================================

✅ Done! Check your AIM dashboard for the audit trail.
```

---

## Step 4: Check Your Dashboard (Team-Level Monitoring)

Open http://localhost:3000 → Agents → research-crew

### Crew Status

```
Agent: research-crew
Type: Multi-Agent Team
Status: ✅ ACTIVE
Trust Score: 0.93 (Excellent)
Last Verified: 2 minutes ago
Total Tasks: 1
Total Agent Actions: 23
```

### Agent Activity Breakdown

```
👤 Researcher (Senior Research Analyst)
   ✅ search_tool("AI healthcare 2025")           |  5m ago  |  SUCCESS
   ✅ scrape_tool("https://healthcare.ai/...")     |  4m ago  |  SUCCESS
   ✅ search_tool("AI diagnosis accuracy")         |  3m ago  |  SUCCESS
   Total Actions: 8

👤 Writer (Tech Content Writer)
   ✅ write_draft(topic="AI Healthcare")           |  2m ago  |  SUCCESS
   ✅ structure_article()                          |  2m ago  |  SUCCESS
   Total Actions: 5

👤 Editor (Content Editor)
   ✅ review_grammar()                             |  1m ago  |  SUCCESS
   ✅ check_facts()                                |  1m ago  |  SUCCESS
   ✅ polish_content()                             |  30s ago |  SUCCESS
   Total Actions: 10
```

### Trust Score Breakdown

```
✅ Verification Status:     100%  (1.00)  [All 23 actions verified]
✅ Uptime & Availability:   100%  (1.00)  [Crew always responsive]
✅ Action Success Rate:      96%  (0.96)  [22/23 succeeded, 1 retry]
✅ Security Alerts:           0   (1.00)  [No anomalies detected]
✅ Compliance Score:        100%  (1.00)  [Following policies]
⚠️  Age & History:          New   (0.50)  [Score improves over time]
✅ Drift Detection:         None  (1.00)  [Consistent behavior]
✅ User Feedback:           None  (1.00)  [No complaints]

Overall Trust Score: 0.93 / 1.00
```

### Complete Audit Trail

```
📝 2025-10-21 15:10:22 UTC  |  Crew registered
📝 2025-10-21 15:15:30 UTC  |  Task started: research-and-write("AI Healthcare 2025")
📝 2025-10-21 15:15:35 UTC  |  [Researcher] search_tool("AI healthcare 2025")
📝 2025-10-21 15:16:12 UTC  |  [Researcher] scrape_tool("https://healthcare.ai/2025")
📝 2025-10-21 15:16:45 UTC  |  [Researcher] search_tool("AI diagnosis accuracy")
📝 2025-10-21 15:17:20 UTC  |  [Researcher] Task completed
📝 2025-10-21 15:17:25 UTC  |  [Writer] write_draft(topic="AI Healthcare")
📝 2025-10-21 15:18:10 UTC  |  [Writer] structure_article()
📝 2025-10-21 15:18:45 UTC  |  [Writer] Task completed
📝 2025-10-21 15:18:50 UTC  |  [Editor] review_grammar()
📝 2025-10-21 15:19:15 UTC  |  [Editor] check_facts()
📝 2025-10-21 15:19:40 UTC  |  [Editor] polish_content()
📝 2025-10-21 15:20:00 UTC  |  [Editor] Task completed
📝 2025-10-21 15:20:02 UTC  |  Crew task completed successfully
```

---

## 🎓 Understanding the Integration

### What Does `AIMCrewWrapper` Do?

```python
secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)
```

Behind this wrapper, AIM:
1. ✅ Intercepts all agent actions (tool calls, delegations)
2. ✅ Verifies each action with cryptographic signatures
3. ✅ Logs complete audit trail (who did what, when)
4. ✅ Monitors for anomalies across all agents
5. ✅ Calculates team-level trust score
6. ✅ Triggers security alerts if needed

### How Are Crew Actions Verified?

Every time an agent in your crew acts:
```python
# Researcher uses search tool
researcher.execute_task(research_task)
  → search_tool("AI healthcare 2025")
```

AIM automatically:
1. **Captures** the action context (agent, tool, parameters)
2. **Signs** the action with Ed25519 private key
3. **Verifies** the signature with AIM platform
4. **Logs** the action with full context
5. **Updates** trust score for the crew
6. **Monitors** for behavioral drift

**Zero code changes to your agents!**

### Team-Level Trust Scoring

Your crew's trust score (0.93) reflects:

1. **Verification Status** (25%): All agent actions verified
2. **Uptime** (15%): Crew always responsive
3. **Success Rate** (15%): 96% of actions succeeded (22/23)
4. **Security Alerts** (15%): Zero alerts
5. **Compliance** (10%): Following all policies
6. **Age** (10%): New crew (improves over time)
7. **Drift Detection** (5%): Consistent behavior patterns
8. **User Feedback** (5%): No negative feedback

---

## 🚀 Advanced Usage

### Multi-Crew Coordination

```python
from aim_sdk import secure
from aim_sdk.integrations.crewai import AIMCrewWrapper
from crewai import Agent, Task, Crew

# Register multiple crews
research_crew_aim = secure("research-crew")
marketing_crew_aim = secure("marketing-crew")

# Define crews
research_crew = Crew(agents=[researcher, analyst], tasks=[research_tasks])
marketing_crew = Crew(agents=[marketer, designer], tasks=[marketing_tasks])

# Secure both crews
secured_research = AIMCrewWrapper(research_crew, aim_agent=research_crew_aim)
secured_marketing = AIMCrewWrapper(marketing_crew, aim_agent=marketing_crew_aim)

# Run crews (both monitored separately)
research_result = secured_research.kickoff(inputs={"topic": "AI Trends"})
marketing_result = secured_marketing.kickoff(inputs={"product": research_result})

# Dashboard shows both crews separately with individual trust scores
```

### Hierarchical Process

```python
from crewai import Process

# Create crew with hierarchical process
crew = Crew(
    agents=[researcher, writer, editor],
    tasks=[research_task, writing_task, editing_task],
    process=Process.hierarchical,  # Manager delegates tasks
    manager_llm=ChatOpenAI(model="gpt-4")
)

# Secure the hierarchical crew
secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# AIM tracks delegation patterns and manager decisions
result = secured_crew.kickoff(inputs={"topic": "Quantum Computing"})
```

### Custom Callbacks

```python
from aim_sdk.integrations.crewai import AIMCrewWrapper

def on_agent_action(agent_name: str, action: str, result: str):
    """Custom callback for each agent action"""
    print(f"[{agent_name}] {action} → {result[:50]}...")

# Secure crew with custom callbacks
secured_crew = AIMCrewWrapper(
    crew,
    aim_agent=aim_crew,
    on_action=on_agent_action
)

result = secured_crew.kickoff(inputs={"topic": "Blockchain"})
# Prints each action as it happens
```

---

## 💡 Real-World Use Cases

### 1. Content Creation Pipeline

```python
from aim_sdk import secure
from crewai import Agent, Task, Crew

aim_crew = secure("content-pipeline")

# Content crew
researcher = Agent(role="Researcher", goal="Research topics", tools=[search_tool])
writer = Agent(role="Writer", goal="Write articles", tools=[write_tool])
seo_optimizer = Agent(role="SEO Expert", goal="Optimize for SEO", tools=[seo_tool])
publisher = Agent(role="Publisher", goal="Publish content", tools=[cms_tool])

crew = Crew(
    agents=[researcher, writer, seo_optimizer, publisher],
    tasks=[research_task, write_task, seo_task, publish_task],
    process=Process.sequential
)

secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# Run pipeline (fully audited)
result = secured_crew.kickoff(inputs={"topic": "AI in 2025"})
```

### 2. Business Intelligence Team

```python
from aim_sdk import secure
from crewai import Agent, Task, Crew

aim_crew = secure("bi-team")

# BI crew
data_analyst = Agent(role="Data Analyst", goal="Analyze data", tools=[sql_tool])
statistician = Agent(role="Statistician", goal="Statistical analysis", tools=[stats_tool])
visualizer = Agent(role="Data Viz Expert", goal="Create charts", tools=[viz_tool])
presenter = Agent(role="Presenter", goal="Create presentation", tools=[ppt_tool])

crew = Crew(
    agents=[data_analyst, statistician, visualizer, presenter],
    tasks=[analyze_task, stats_task, viz_task, present_task]
)

secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# Run BI pipeline
report = secured_crew.kickoff(inputs={"dataset": "Q4_sales.csv"})
```

### 3. Customer Support Crew

```python
from aim_sdk import secure
from crewai import Agent, Task, Crew

aim_crew = secure("support-crew")

# Support crew
ticket_analyzer = Agent(role="Ticket Analyzer", goal="Categorize tickets", tools=[nlp_tool])
knowledge_base = Agent(role="KB Searcher", goal="Find solutions", tools=[kb_tool])
responder = Agent(role="Support Agent", goal="Draft responses", tools=[email_tool])
quality_checker = Agent(role="QA", goal="Review responses", tools=[qa_tool])

crew = Crew(
    agents=[ticket_analyzer, knowledge_base, responder, quality_checker],
    tasks=[analyze_task, search_task, respond_task, qa_task]
)

secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# Process support ticket
response = secured_crew.kickoff(inputs={"ticket_id": "T-12345"})
```

---

## 🐛 Troubleshooting

### Issue: "Agent not found in AIM"

**Error**: `AIMError: Agent 'research-crew' not registered`

**Solution**:
1. Register crew in AIM dashboard first
2. Verify `AIM_PRIVATE_KEY` is set correctly
3. Check AIM backend is running: `curl http://localhost:8080/health`

### Issue: "CrewAI tools not verified"

**Error**: `Tool execution not logged in AIM`

**Solution**:
- Ensure you're using `AIMCrewWrapper`, not plain `Crew`
- Verify wrapper initialization: `secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)`
- Check AIM dashboard for connection status

### Issue: "Low trust score for crew"

**Symptoms**: Trust score below 0.70

**Common Causes**:
1. **High failure rate**: Some agents failing frequently
2. **Anomalies detected**: Unusual behavior patterns
3. **Security alerts**: Suspicious tool usage

**Solution**:
```python
# Check crew status in dashboard
# Review failed actions in audit trail
# Investigate agents with low success rates
# Check for tools being misused
```

---

## ✅ Checklist

- [ ] Crew registered in AIM dashboard
- [ ] Private key saved securely
- [ ] CrewAI installed (`pip install crewai`)
- [ ] AIM SDK downloaded from dashboard
- [ ] SDK dependencies installed (`pip install keyring PyNaCl requests cryptography`)
- [ ] Code uses `AIMCrewWrapper`
- [ ] Crew runs without errors
- [ ] Dashboard shows crew status
- [ ] Trust score visible (should be >0.90)
- [ ] Agent actions logged in audit trail
- [ ] No security alerts

**All checked?** 🎉 **Your CrewAI team is secure!**

---

## 🚀 Next Steps

### Explore More Integrations

- [LangChain Integration →](./langchain.md) - Secure LangChain agents
- [MCP Integration →](./mcp.md) - Register MCP servers
- [Microsoft Copilot →](./copilot.md) - AI assistants

### Learn Advanced Features

- [SDK Documentation](../sdk/python.md) - Complete SDK reference
- [Trust Scoring](../sdk/trust-scoring.md) - Team trust algorithms
- [Audit & Compliance](../security/audit-logs.md) - SOC 2 compliance

### Deploy to Production

- [Azure Deployment](../deployment/azure.md) - Production setup
- [Security Best Practices](../security/best-practices.md) - Harden deployment

---

<div align="center">

**Next**: [MCP Integration →](./mcp.md)

[🏠 Back to Home](../../README.md) • [📚 All Integrations](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
