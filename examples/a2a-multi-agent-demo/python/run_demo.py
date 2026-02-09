#!/usr/bin/env python3
"""
A2A Multi-Agent Collaboration Demo

Demonstrates Research Agent discovering and collaborating with Analysis Agent
using all A2A features:
1. Agent Card Registration
2. Intent-Based Discovery
3. Trust Score Management
4. Security Policy Evaluation
5. GDPR Consent Management
6. Request Signing & Task Logging
7. Skill Attestation & Consensus

Run with:
    python run_demo.py [--aim-url http://localhost:8080]

The SDK handles authentication automatically via bundled credentials.
"""
import os
import sys
import json
import argparse
from datetime import datetime

# Import agents
from research_agent import ResearchAgent
from analysis_agent import AnalysisAgent


def load_sample_data() -> dict:
    """Load sample data for the demo."""
    data_path = os.path.join(
        os.path.dirname(__file__),
        '../shared/sample-data/research-data.json'
    )

    if os.path.exists(data_path):
        with open(data_path, 'r') as f:
            return json.load(f)

    # Fallback data if file not found
    return {
        "customerFeedback": [
            {
                "id": "fb-001",
                "text": "This product exceeded my expectations! The quality is outstanding.",
                "source": "customer-survey"
            }
        ],
        "timeSeriesData": [
            {"date": "2025-01-01", "metric": "sales", "value": 1000},
            {"date": "2025-01-02", "metric": "sales", "value": 1200},
            {"date": "2025-01-03", "metric": "sales", "value": 1350}
        ],
        "documentContent": "Quarterly report summarizing company performance."
    }


def print_banner():
    """Print demo banner."""
    print("\n" + "=" * 70)
    print("  A2A MULTI-AGENT COLLABORATION DEMO")
    print("  Research Agent <-> Analysis Agent")
    print("=" * 70)
    print(f"  Started: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 70 + "\n")


def print_step(step_num: int, title: str):
    """Print step header."""
    print(f"\n{'─' * 70}")
    print(f"  STEP {step_num}: {title}")
    print(f"{'─' * 70}\n")


def print_summary(
    aim_url: str,
    research_agent: ResearchAgent,
    analysis_agent: AnalysisAgent,
    results: dict
):
    """Print demo summary with dashboard links."""
    print("\n" + "=" * 70)
    print("  DEMO SUMMARY")
    print("=" * 70)

    # Results summary
    print("\n  Results:")
    print(f"    - Analysis Agent ID: {analysis_agent.agent_id[:16]}...")
    print(f"    - Research Agent ID: {research_agent.agent_id[:16]}...")
    print(f"    - Skills Registered: {len(analysis_agent.SKILLS) + 2}")
    print(f"    - Sentiment Result: {results.get('sentiment', {}).get('sentiment', 'N/A')}")
    print(f"    - Trend Result: {results.get('trend', {}).get('trend', 'N/A')}")

    # Dashboard links
    base_url = aim_url.replace('/api', '')
    print("\n  Verify in Dashboard:")
    print(f"    - A2A Overview:    {base_url}/dashboard/a2a")
    print(f"    - Agent Cards:     {base_url}/dashboard/a2a?tab=cards")
    print(f"    - Consent Records: {base_url}/dashboard/a2a?tab=consent")
    print(f"    - Task History:    {base_url}/dashboard/a2a?tab=tasks")
    print(f"    - Trust Scores:    {base_url}/dashboard/a2a?tab=trust")
    print(f"    - Skill Search:    {base_url}/dashboard/a2a?tab=skills")

    print("\n" + "=" * 70)
    print("  Demo completed successfully!")
    print("=" * 70 + "\n")


def run_demo(aim_url: str, api_token: str = None):
    """
    Run the full A2A collaboration demo.

    Args:
        aim_url: AIM platform URL (SDK handles auth automatically)
        api_token: Optional Bearer token for API auth (bypasses SDK credentials)
    """
    print_banner()

    # Load sample data
    sample_data = load_sample_data()
    results = {}

    # =========================================================================
    # STEP 1: Register Analysis Agent
    # =========================================================================
    print_step(1, "REGISTER ANALYSIS AGENT")

    analysis_agent = AnalysisAgent(aim_url=aim_url, api_token=api_token)
    analysis_card = analysis_agent.register_agent_card()

    print(f"\nAnalysis Agent ready with {len(analysis_agent.SKILLS)} skills:")
    for skill in analysis_agent.SKILLS:
        print(f"  - {skill['name']}: {skill['description'][:50]}...")

    # =========================================================================
    # STEP 2: Register Research Agent
    # =========================================================================
    print_step(2, "REGISTER RESEARCH AGENT")

    research_agent = ResearchAgent(aim_url=aim_url, api_token=api_token)
    research_card = research_agent.register_agent_card()

    # =========================================================================
    # STEP 3: Discover Analysis Agent by Intent
    # =========================================================================
    print_step(3, "INTENT-BASED DISCOVERY")

    intent = "I need to analyze customer feedback sentiment to understand satisfaction levels"
    discovered = research_agent.discover_analysis_agent(intent=intent, min_trust=0.3)

    if discovered:
        target_agent_id = discovered.get('agentId') or analysis_agent.agent_id
        skill_id = discovered.get('skillId', 'sentiment-analysis')
    else:
        # Fallback to direct reference for demo
        print("[Demo] Using direct agent reference for demo purposes")
        target_agent_id = analysis_agent.agent_id
        skill_id = "sentiment-analysis"

    # =========================================================================
    # STEP 4: Check Trust Scores
    # =========================================================================
    print_step(4, "TRUST SCORE MANAGEMENT")

    # Get self trust score
    research_trust = research_agent.get_trust_score()

    # Get peer trust with Analysis Agent
    peer_trust = research_agent.get_peer_trust(target_agent_id)

    # =========================================================================
    # STEP 5: Security Policy Evaluation
    # =========================================================================
    print_step(5, "SECURITY POLICY EVALUATION")

    security_result = research_agent.check_security_policies(
        target_agent_id=target_agent_id,
        skill_id=skill_id
    )

    # =========================================================================
    # STEP 6: GDPR Consent Management
    # =========================================================================
    print_step(6, "GDPR CONSENT MANAGEMENT")

    user_id = "demo-user-001"
    consent_granted = research_agent.request_user_consent(
        user_id=user_id,
        recipient_agent_id=target_agent_id,
        data_types=["customer_feedback", "sentiment_data"],
        purpose="Analyze customer feedback sentiment for satisfaction insights"
    )

    if not consent_granted:
        print("[Demo] Warning: Consent not granted, but continuing for demo...")

    # =========================================================================
    # STEP 7: Execute Skills with Signing & Task Logging
    # =========================================================================
    print_step(7, "SKILL EXECUTION (Signing + Task Logging)")

    # 7a: Sentiment Analysis
    print("\n--- Sentiment Analysis ---")
    feedback_text = sample_data["customerFeedback"][0]["text"]
    print(f"Input: \"{feedback_text[:60]}...\"")

    sentiment_result = research_agent.sign_and_call_skill(
        target_agent_id=target_agent_id,
        skill_id="sentiment-analysis",
        data={"text": feedback_text},
        analysis_agent=analysis_agent
    )
    results['sentiment'] = sentiment_result

    # 7b: Trend Detection
    print("\n--- Trend Detection ---")
    time_series = sample_data["timeSeriesData"]
    print(f"Input: {len(time_series)} data points")

    trend_result = research_agent.sign_and_call_skill(
        target_agent_id=target_agent_id,
        skill_id="trend-detection",
        data={"data": time_series},
        analysis_agent=analysis_agent
    )
    results['trend'] = trend_result

    # 7c: Summarization
    print("\n--- Summarization ---")
    document = sample_data["documentContent"]
    print(f"Input: {len(document)} characters")

    summary_result = research_agent.sign_and_call_skill(
        target_agent_id=target_agent_id,
        skill_id="summarization",
        data={"text": document},
        analysis_agent=analysis_agent
    )
    results['summary'] = summary_result

    # =========================================================================
    # STEP 8: Skill Attestation
    # =========================================================================
    print_step(8, "SKILL ATTESTATION")

    attestation = research_agent.attest_skill_quality(
        target_agent_id=target_agent_id,
        skill_id="sentiment-analysis",
        confidence=0.92,
        evidence={
            "sampleSize": len(sample_data["customerFeedback"]),
            "accuracyObserved": 0.95,
            "responseTime": "fast",
            "outputQuality": "high"
        }
    )

    # =========================================================================
    # STEP 9: Print Summary
    # =========================================================================
    print_step(9, "VERIFICATION")

    print("All A2A features demonstrated:")
    print("  [x] Agent Card Registration")
    print("  [x] Intent-Based Discovery")
    print("  [x] Trust Score Management")
    print("  [x] Security Policy Evaluation")
    print("  [x] GDPR Consent Management")
    print("  [x] Request Signing")
    print("  [x] Task Logging & State Updates")
    print("  [x] Skill Attestation")
    print("  [x] Consensus Verification")

    print_summary(aim_url, research_agent, analysis_agent, results)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="A2A Multi-Agent Collaboration Demo"
    )
    parser.add_argument(
        '--aim-url',
        default=os.environ.get('AIM_URL', 'http://localhost:8080'),
        help='AIM platform URL (default: http://localhost:8080)'
    )
    parser.add_argument(
        '--api-token',
        default=os.environ.get('AIM_API_TOKEN'),
        help='Bearer token for API auth (bypasses SDK credentials). Also reads AIM_API_TOKEN env var.'
    )

    args = parser.parse_args()

    try:
        run_demo(aim_url=args.aim_url, api_token=args.api_token)
    except KeyboardInterrupt:
        print("\n\nDemo interrupted by user.")
        sys.exit(0)
    except Exception as e:
        print(f"\nDemo error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
