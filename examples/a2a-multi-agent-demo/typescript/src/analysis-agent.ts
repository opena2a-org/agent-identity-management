/**
 * Analysis Agent - Provides data analysis skills for A2A collaboration.
 */
import { AIMClient, A2AClient, AgentType } from '@opena2a/aim-sdk';

interface SentimentResult {
  sentiment: string;
  confidence: number;
  wordCount: number;
  positiveIndicators: number;
  negativeIndicators: number;
}

interface SummaryResult {
  summary: string;
  originalLength: number;
  summaryLength: number;
  compressionRatio: number;
  sentenceCount: number;
}

interface TrendResult {
  trend: string;
  changePercent?: number;
  dataPoints: number;
  startValue?: number;
  endValue?: number;
  confidence: number;
}

interface SkillDefinition {
  name: string;
  description: string;
  tags: string[];
}

/**
 * Analysis Agent providing sentiment analysis, summarization, and trend detection.
 */
export class AnalysisAgent {
  static readonly SKILLS: SkillDefinition[] = [
    {
      name: 'Sentiment Analysis',
      description: 'Analyze sentiment of text data, returning positive/negative/neutral classification',
      tags: ['nlp', 'sentiment', 'text-analysis', 'customer-feedback'],
    },
    {
      name: 'Text Summarization',
      description: 'Summarize long text documents into concise summaries',
      tags: ['nlp', 'summarization', 'text', 'content'],
    },
    {
      name: 'Trend Detection',
      description: 'Detect trends and patterns in time-series data',
      tags: ['analytics', 'trends', 'data-analysis', 'forecasting'],
    },
  ];

  private readonly agentName = 'analysis-agent';
  private aimClient: AIMClient;
  private a2a!: A2AClient;
  public agentId: string = '';

  constructor(private readonly baseUrl?: string) {
    console.log('[Analysis Agent] Initializing...');

    // SDK handles auth automatically via bundled credentials
    this.aimClient = new AIMClient({
      baseUrl: baseUrl || 'http://localhost:8080',
    });
  }

  async initialize(): Promise<void> {
    console.log('[Analysis Agent] Registering with AIM...');

    const agent = await this.aimClient.registerAgent({
      name: this.agentName,
      displayName: 'Analysis Agent',
      agentType: AgentType.AI_AGENT,
      description: 'Analysis agent providing sentiment analysis, summarization, and trend detection',
    });

    this.agentId = agent.id;
    this.a2a = new A2AClient(this.aimClient, this.agentId);
    console.log(`[Analysis Agent] Registered with ID: ${this.agentId.slice(0, 8)}...`);
  }

  async registerAgentCard(): Promise<Record<string, unknown> | null> {
    console.log(`[Analysis Agent] Registering skills (${AnalysisAgent.SKILLS.length})...`);

    try {
      for (const skill of AnalysisAgent.SKILLS) {
        await this.a2a.registerSkill({
          name: skill.name,
          description: skill.description,
          tags: skill.tags,
        });
        console.log(`  - Registered skill: ${skill.name}`);
      }

      console.log('[Analysis Agent] Skills registered successfully');
      return { skills: AnalysisAgent.SKILLS.length };
    } catch (e) {
      console.log(`[Analysis Agent] Registration note: ${e}`);
      return null;
    }
  }

  executeSkill(
    skillId: string,
    inputData: Record<string, unknown>
  ): SentimentResult | SummaryResult | TrendResult | { error: string } {
    console.log(`[Analysis Agent] Executing skill: ${skillId}`);

    switch (skillId) {
      case 'sentiment-analysis':
        return this.analyzeSentiment((inputData.text as string) || '');
      case 'summarization':
        return this.summarize((inputData.text as string) || '');
      case 'trend-detection':
        return this.detectTrends((inputData.data as Array<Record<string, unknown>>) || []);
      default:
        return { error: `Unknown skill: ${skillId}` };
    }
  }

  private analyzeSentiment(text: string): SentimentResult {
    const positiveWords = ['excellent', 'great', 'amazing', 'good', 'outstanding', 'recommend', 'happy', 'love'];
    const negativeWords = ['bad', 'terrible', 'awful', 'disappointed', 'not satisfied', 'poor', 'hate'];

    const textLower = text.toLowerCase();
    const positiveCount = positiveWords.filter((w) => textLower.includes(w)).length;
    const negativeCount = negativeWords.filter((w) => textLower.includes(w)).length;

    let sentiment: string;
    let confidence: number;

    if (positiveCount > negativeCount) {
      sentiment = 'positive';
      confidence = Math.min(0.5 + positiveCount * 0.1, 0.95);
    } else if (negativeCount > positiveCount) {
      sentiment = 'negative';
      confidence = Math.min(0.5 + negativeCount * 0.1, 0.95);
    } else {
      sentiment = 'neutral';
      confidence = 0.6;
    }

    return {
      sentiment,
      confidence: Math.round(confidence * 100) / 100,
      wordCount: text.split(/\s+/).length,
      positiveIndicators: positiveCount,
      negativeIndicators: negativeCount,
    };
  }

  private summarize(text: string): SummaryResult {
    const sentences = text.split('.').map((s) => s.trim()).filter((s) => s.length > 0);
    const summarySentences = sentences.length > 2 ? sentences.slice(0, 2) : sentences;
    const summary = summarySentences.length > 0 ? summarySentences.join('. ') + '.' : text;

    return {
      summary,
      originalLength: text.length,
      summaryLength: summary.length,
      compressionRatio: Math.round((summary.length / Math.max(text.length, 1)) * 100) / 100,
      sentenceCount: sentences.length,
    };
  }

  private detectTrends(data: Array<Record<string, unknown>>): TrendResult {
    if (!data || data.length < 2) {
      return { trend: 'insufficient_data', dataPoints: data?.length || 0, confidence: 0.0 };
    }

    const values = data.filter((d) => typeof d === 'object' && d !== null).map((d) => (d.value as number) || 0);

    if (values.length < 2) {
      return { trend: 'insufficient_data', dataPoints: values.length, confidence: 0.0 };
    }

    const halfIndex = Math.floor(values.length / 2);
    const firstHalfAvg = values.slice(0, halfIndex).reduce((a, b) => a + b, 0) / halfIndex;
    const secondHalfAvg = values.slice(halfIndex).reduce((a, b) => a + b, 0) / (values.length - halfIndex);
    const changePct = firstHalfAvg !== 0 ? ((secondHalfAvg - firstHalfAvg) / firstHalfAvg) * 100 : 0;

    let trend: string;
    if (changePct > 5) trend = 'upward';
    else if (changePct < -5) trend = 'downward';
    else trend = 'stable';

    return {
      trend,
      changePercent: Math.round(changePct * 10) / 10,
      dataPoints: values.length,
      startValue: values[0],
      endValue: values[values.length - 1],
      confidence: 0.78,
    };
  }

  async getTrustScore(): Promise<number> {
    try {
      const score = await this.a2a.getTrustScore(this.agentId);
      return score.score;
    } catch {
      return 0.0;
    }
  }
}
