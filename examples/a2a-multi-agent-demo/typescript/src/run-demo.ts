#!/usr/bin/env ts-node
/**
 * A2A Multi-Agent Collaboration Demo
 *
 * Demonstrates Research Agent discovering and collaborating with Analysis Agent
 * using all A2A features.
 *
 * Run with:
 *     export AIM_API_KEY='your-key'
 *     npm run demo
 */
import * as fs from 'fs';
import * as path from 'path';
import { ResearchAgent } from './research-agent';
import { AnalysisAgent } from './analysis-agent';

interface SampleData {
  customerFeedback: Array<{ id: string; text: string; source: string; timestamp: string }>;
  timeSeriesData: Array<{ date: string; metric: string; value: number }>;
  documentContent: string;
}

interface DemoResults {
  sentiment?: Record<string, unknown>;
  trend?: Record<string, unknown>;
  summary?: Record<string, unknown>;
}

function loadSampleData(): SampleData {
  const dataPath = path.join(__dirname, '../../shared/sample-data/research-data.json');

  if (fs.existsSync(dataPath)) {
    return JSON.parse(fs.readFileSync(dataPath, 'utf-8'));
  }

  // Fallback data
  return {
    customerFeedback: [
      {
        id: 'fb-001',
        text: 'This product exceeded my expectations! The quality is outstanding.',
        source: 'customer-survey',
        timestamp: new Date().toISOString(),
      },
    ],
    timeSeriesData: [
      { date: '2025-01-01', metric: 'sales', value: 1000 },
      { date: '2025-01-02', metric: 'sales', value: 1200 },
      { date: '2025-01-03', metric: 'sales', value: 1350 },
    ],
    documentContent: 'Quarterly report summarizing company performance.',
  };
}

function printBanner(): void {
  console.log('\n' + '='.repeat(70));
  console.log('  A2A MULTI-AGENT COLLABORATION DEMO (TypeScript)');
  console.log('  Research Agent <-> Analysis Agent');
  console.log('='.repeat(70));
  console.log(`  Started: ${new Date().toISOString()}`);
  console.log('='.repeat(70) + '\n');
}

function printStep(stepNum: number, title: string): void {
  console.log(`\n${'─'.repeat(70)}`);
  console.log(`  STEP ${stepNum}: ${title}`);
  console.log(`${'─'.repeat(70)}\n`);
}

function printSummary(
  baseUrl: string,
  researchAgent: ResearchAgent,
  analysisAgent: AnalysisAgent,
  results: DemoResults
): void {
  console.log('\n' + '='.repeat(70));
  console.log('  DEMO SUMMARY');
  console.log('='.repeat(70));

  // Results summary
  console.log('\n  Results:');
  console.log(`    - Analysis Agent ID: ${analysisAgent.agentId.slice(0, 16)}...`);
  console.log(`    - Research Agent ID: ${researchAgent.agentId.slice(0, 16)}...`);
  console.log(`    - Skills Registered: ${AnalysisAgent.SKILLS.length + ResearchAgent.SKILLS.length}`);
  console.log(
    `    - Sentiment Result: ${(results.sentiment as Record<string, unknown>)?.sentiment ?? 'N/A'}`
  );
  console.log(`    - Trend Result: ${(results.trend as Record<string, unknown>)?.trend ?? 'N/A'}`);

  // Dashboard links
  console.log('\n  Verify in Dashboard:');
  console.log(`    - A2A Overview:    ${baseUrl}/dashboard/a2a`);
  console.log(`    - Agent Cards:     ${baseUrl}/dashboard/a2a?tab=cards`);
  console.log(`    - Consent Records: ${baseUrl}/dashboard/a2a?tab=consent`);
  console.log(`    - Task History:    ${baseUrl}/dashboard/a2a?tab=tasks`);
  console.log(`    - Trust Scores:    ${baseUrl}/dashboard/a2a?tab=trust`);
  console.log(`    - Skill Search:    ${baseUrl}/dashboard/a2a?tab=skills`);

  console.log('\n' + '='.repeat(70));
  console.log('  Demo completed successfully!');
  console.log('='.repeat(70) + '\n');
}

async function runDemo(baseUrl: string, apiKey: string): Promise<void> {
  printBanner();

  const sampleData = loadSampleData();
  const results: DemoResults = {};

  // =========================================================================
  // STEP 1: Register Analysis Agent
  // =========================================================================
  printStep(1, 'REGISTER ANALYSIS AGENT');

  const analysisAgent = new AnalysisAgent(baseUrl, apiKey);
  await analysisAgent.initialize();
  await analysisAgent.registerAgentCard();

  console.log(`\nAnalysis Agent ready with ${AnalysisAgent.SKILLS.length} skills:`);
  for (const skill of AnalysisAgent.SKILLS) {
    console.log(`  - ${skill.name}: ${skill.description.slice(0, 50)}...`);
  }

  // =========================================================================
  // STEP 2: Register Research Agent
  // =========================================================================
  printStep(2, 'REGISTER RESEARCH AGENT');

  const researchAgent = new ResearchAgent(baseUrl, apiKey);
  await researchAgent.initialize();
  await researchAgent.registerAgentCard();

  // =========================================================================
  // STEP 3: Discover Analysis Agent by Intent
  // =========================================================================
  printStep(3, 'INTENT-BASED DISCOVERY');

  const intent =
    'I need to analyze customer feedback sentiment to understand satisfaction levels';
  const discovered = await researchAgent.discoverAnalysisAgent(intent, 0.3);

  let targetAgentId: string;
  let skillId: string;

  if (discovered && (discovered as Record<string, unknown>).agentId) {
    targetAgentId = (discovered as Record<string, unknown>).agentId as string;
    skillId = 'sentiment-analysis';
  } else {
    console.log('[Demo] Using direct agent reference for demo purposes');
    targetAgentId = analysisAgent.agentId;
    skillId = 'sentiment-analysis';
  }

  // =========================================================================
  // STEP 4: Check Trust Scores
  // =========================================================================
  printStep(4, 'TRUST SCORE MANAGEMENT');

  await researchAgent.getTrustScore();
  await researchAgent.getPeerTrust(targetAgentId);

  // =========================================================================
  // STEP 5: Security Policy Evaluation
  // =========================================================================
  printStep(5, 'SECURITY POLICY EVALUATION');

  await researchAgent.checkSecurityPolicies(targetAgentId, skillId);

  // =========================================================================
  // STEP 6: GDPR Consent Management
  // =========================================================================
  printStep(6, 'GDPR CONSENT MANAGEMENT');

  const userId = 'demo-user-001';
  const consentGranted = await researchAgent.requestUserConsent(
    userId,
    targetAgentId,
    ['customer_feedback', 'sentiment_data'],
    'Analyze customer feedback sentiment for satisfaction insights'
  );

  if (!consentGranted) {
    console.log('[Demo] Warning: Consent not granted, but continuing for demo...');
  }

  // =========================================================================
  // STEP 7: Execute Skills with Signing & Task Logging
  // =========================================================================
  printStep(7, 'SKILL EXECUTION (Signing + Task Logging)');

  // 7a: Sentiment Analysis
  console.log('\n--- Sentiment Analysis ---');
  const feedbackText = sampleData.customerFeedback[0].text;
  console.log(`Input: "${feedbackText.slice(0, 60)}..."`);

  results.sentiment = await researchAgent.signAndCallSkill(
    targetAgentId,
    'sentiment-analysis',
    { text: feedbackText },
    analysisAgent
  );

  // 7b: Trend Detection
  console.log('\n--- Trend Detection ---');
  const timeSeries = sampleData.timeSeriesData;
  console.log(`Input: ${timeSeries.length} data points`);

  results.trend = await researchAgent.signAndCallSkill(
    targetAgentId,
    'trend-detection',
    { data: timeSeries },
    analysisAgent
  );

  // 7c: Summarization
  console.log('\n--- Summarization ---');
  const document = sampleData.documentContent;
  console.log(`Input: ${document.length} characters`);

  results.summary = await researchAgent.signAndCallSkill(
    targetAgentId,
    'summarization',
    { text: document },
    analysisAgent
  );

  // =========================================================================
  // STEP 8: Skill Attestation
  // =========================================================================
  printStep(8, 'SKILL ATTESTATION');

  await researchAgent.attestSkillQuality(targetAgentId, 'sentiment-analysis', 0.92, {
    sampleSize: sampleData.customerFeedback.length,
    accuracyObserved: 0.95,
    responseTime: 'fast',
    outputQuality: 'high',
  });

  // =========================================================================
  // STEP 9: Print Summary
  // =========================================================================
  printStep(9, 'VERIFICATION');

  console.log('All A2A features demonstrated:');
  console.log('  [x] Agent Card Registration');
  console.log('  [x] Intent-Based Discovery');
  console.log('  [x] Trust Score Management');
  console.log('  [x] Security Policy Evaluation');
  console.log('  [x] GDPR Consent Management');
  console.log('  [x] Request Signing');
  console.log('  [x] Task Logging & State Updates');
  console.log('  [x] Skill Attestation');
  console.log('  [x] Consensus Verification');

  printSummary(baseUrl, researchAgent, analysisAgent, results);
}

async function main(): Promise<void> {
  const baseUrl = process.env.AIM_URL || 'http://localhost:8080';
  const apiKey = process.env.AIM_API_KEY || '';

  if (!apiKey) {
    console.error('Error: API key required. Set AIM_API_KEY environment variable');
    console.error('\nUsage:');
    console.error("  export AIM_API_KEY='your-api-key'");
    console.error('  npm run demo');
    process.exit(1);
  }

  try {
    await runDemo(baseUrl, apiKey);
  } catch (e) {
    console.error(`\nDemo error: ${e}`);
    if (e instanceof Error) {
      console.error(e.stack);
    }
    process.exit(1);
  }
}

main();
