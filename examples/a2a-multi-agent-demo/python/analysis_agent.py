"""
Analysis Agent - Provides data analysis skills for A2A collaboration.

Demonstrates:
- Agent registration with AIM
- Agent Card with multiple skills
- Request verification
- Skill execution
"""
import os
import sys
from typing import Dict, List, Optional

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../../sdk/python'))

from aim_sdk import register_agent
from aim_sdk.a2a import A2AClient


class AnalysisAgent:
    """
    Analysis Agent providing sentiment analysis, summarization, and trend detection.

    This agent registers with AIM and exposes skills that other agents can discover
    and invoke through the A2A protocol.
    """

    SKILLS = [
        {
            "id": "sentiment-analysis",
            "name": "Sentiment Analysis",
            "description": "Analyze sentiment of text data, returning positive/negative/neutral classification",
            "tags": ["nlp", "sentiment", "text-analysis", "customer-feedback"],
            "inputModes": ["text"],
            "outputModes": ["text"]
        },
        {
            "id": "summarization",
            "name": "Text Summarization",
            "description": "Summarize long text documents into concise summaries",
            "tags": ["nlp", "summarization", "text", "content"],
            "inputModes": ["text"],
            "outputModes": ["text"]
        },
        {
            "id": "trend-detection",
            "name": "Trend Detection",
            "description": "Detect trends and patterns in time-series data",
            "tags": ["analytics", "trends", "data-analysis", "forecasting"],
            "inputModes": ["data"],
            "outputModes": ["data", "text"]
        }
    ]

    def __init__(self, aim_url: str, api_key: str):
        """
        Initialize the Analysis Agent.

        Args:
            aim_url: AIM platform URL
            api_key: API key for authentication
        """
        self.agent_name = "analysis-agent"
        self.aim_url = aim_url

        print(f"[Analysis Agent] Registering with AIM...")

        # Register agent and get AIM client
        self.aim_client = register_agent(
            name=self.agent_name,
            aim_url=aim_url,
            api_key=api_key,
            display_name="Analysis Agent",
            agent_type="ai_agent",
            description="Analysis agent providing sentiment analysis, summarization, and trend detection"
        )

        # Initialize A2A client
        self.a2a = A2AClient(self.aim_client)

        # Store agent ID
        self.agent_id = self.aim_client.agent_id
        print(f"[Analysis Agent] Registered with ID: {self.agent_id[:8]}...")

    def register_agent_card(self) -> Optional[Dict]:
        """
        Register the A2A Agent Card with skills.

        Returns:
            Registered card info or None on failure
        """
        print(f"[Analysis Agent] Registering Agent Card with {len(self.SKILLS)} skills...")

        try:
            # Register skills
            for skill in self.SKILLS:
                self.a2a.register_skill(skill)
                print(f"  - Registered skill: {skill['name']}")

            # Get the registered card
            card = self.a2a.get_agent_card()
            print(f"[Analysis Agent] Agent Card registered successfully")
            return card

        except Exception as e:
            print(f"[Analysis Agent] Card registration note: {e}")
            return None

    def verify_incoming_request(
        self,
        agent_id: str,
        timestamp: int,
        nonce: str,
        signature: str,
        request_hash: str
    ) -> Dict:
        """
        Verify an incoming A2A request signature.

        Args:
            agent_id: Requesting agent's ID
            timestamp: Request timestamp
            nonce: Request nonce
            signature: Ed25519 signature
            request_hash: Hash of request body

        Returns:
            Verification result with agent info and trust score
        """
        print(f"[Analysis Agent] Verifying request from {agent_id[:8]}...")

        result = self.a2a.verify_request(
            agent_id=agent_id,
            timestamp=timestamp,
            nonce=nonce,
            signature=signature,
            request_hash=request_hash
        )

        if result.get("valid"):
            print(f"[Analysis Agent] Request verified! Agent: {result.get('agentName')}")
            print(f"[Analysis Agent] Trust score: {result.get('trustScore', 'N/A')}")
        else:
            print(f"[Analysis Agent] Verification FAILED: {result.get('error')}")

        return result

    def execute_skill(self, skill_id: str, input_data: Dict) -> Dict:
        """
        Execute a skill and return results.

        Args:
            skill_id: ID of skill to execute
            input_data: Input data for the skill

        Returns:
            Skill execution result
        """
        print(f"[Analysis Agent] Executing skill: {skill_id}")

        if skill_id == "sentiment-analysis":
            return self._analyze_sentiment(input_data.get("text", ""))
        elif skill_id == "summarization":
            return self._summarize(input_data.get("text", ""))
        elif skill_id == "trend-detection":
            return self._detect_trends(input_data.get("data", []))
        else:
            return {"error": f"Unknown skill: {skill_id}"}

    def _analyze_sentiment(self, text: str) -> Dict:
        """
        Analyze sentiment of text.

        In production, this would use an NLP model.
        """
        # Simple keyword-based sentiment (demo purposes)
        positive_words = ["excellent", "great", "amazing", "good", "outstanding", "recommend", "happy", "love"]
        negative_words = ["bad", "terrible", "awful", "disappointed", "not satisfied", "poor", "hate"]

        text_lower = text.lower()
        positive_count = sum(1 for word in positive_words if word in text_lower)
        negative_count = sum(1 for word in negative_words if word in text_lower)

        if positive_count > negative_count:
            sentiment = "positive"
            confidence = min(0.5 + (positive_count * 0.1), 0.95)
        elif negative_count > positive_count:
            sentiment = "negative"
            confidence = min(0.5 + (negative_count * 0.1), 0.95)
        else:
            sentiment = "neutral"
            confidence = 0.6

        return {
            "sentiment": sentiment,
            "confidence": round(confidence, 2),
            "wordCount": len(text.split()),
            "positiveIndicators": positive_count,
            "negativeIndicators": negative_count
        }

    def _summarize(self, text: str) -> Dict:
        """
        Summarize text content.

        In production, this would use a summarization model.
        """
        sentences = [s.strip() for s in text.split('.') if s.strip()]

        # Take first 2 sentences as summary (demo purposes)
        summary_sentences = sentences[:2] if len(sentences) > 2 else sentences
        summary = '. '.join(summary_sentences) + '.' if summary_sentences else text

        return {
            "summary": summary,
            "originalLength": len(text),
            "summaryLength": len(summary),
            "compressionRatio": round(len(summary) / max(len(text), 1), 2),
            "sentenceCount": len(sentences)
        }

    def _detect_trends(self, data: List) -> Dict:
        """
        Detect trends in time-series data.

        In production, this would use statistical analysis.
        """
        if not data or len(data) < 2:
            return {
                "trend": "insufficient_data",
                "dataPoints": len(data) if data else 0,
                "confidence": 0.0
            }

        # Extract values
        values = [d.get("value", 0) for d in data if isinstance(d, dict)]

        if len(values) < 2:
            return {
                "trend": "insufficient_data",
                "dataPoints": len(values),
                "confidence": 0.0
            }

        # Simple trend detection
        first_half = sum(values[:len(values)//2]) / (len(values)//2)
        second_half = sum(values[len(values)//2:]) / (len(values) - len(values)//2)

        change_pct = ((second_half - first_half) / first_half) * 100 if first_half != 0 else 0

        if change_pct > 5:
            trend = "upward"
        elif change_pct < -5:
            trend = "downward"
        else:
            trend = "stable"

        return {
            "trend": trend,
            "changePercent": round(change_pct, 1),
            "dataPoints": len(values),
            "startValue": values[0],
            "endValue": values[-1],
            "confidence": 0.78
        }

    def get_skills(self) -> List[Dict]:
        """Return list of available skills."""
        return self.SKILLS

    def get_trust_score(self) -> float:
        """Get the agent's A2A trust score."""
        try:
            score = self.a2a.get_a2a_trust_score()
            return score.a2a_trust_score
        except Exception:
            return 0.0
