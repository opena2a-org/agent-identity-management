import type { LLMAdapter, LLMResponse, LLMAdapterType } from '../types';
import * as https from 'https';
import { nodeRequire } from '../node-require';

/**
 * Anthropic adapter — uses Messages API with Haiku for cheapest assessments.
 * Reads ANTHROPIC_API_KEY from environment.
 */
export class AnthropicAdapter implements LLMAdapter {
  readonly name = 'anthropic';
  private readonly model: string;
  private readonly apiKey: string;

  constructor(config?: Record<string, unknown>) {
    this.model = (config?.model as string) ?? 'claude-haiku-4-5-20251001';
    this.apiKey = (config?.apiKey as string) ?? process.env.ANTHROPIC_API_KEY ?? '';
  }

  async assess(prompt: string, maxTokens: number): Promise<LLMResponse> {
    const body = JSON.stringify({
      model: this.model,
      max_tokens: maxTokens,
      messages: [{ role: 'user', content: prompt }],
    });

    const response = await httpPost('api.anthropic.com', '/v1/messages', body, {
      'x-api-key': this.apiKey,
      'anthropic-version': '2023-06-01',
      'content-type': 'application/json',
    });

    const data = JSON.parse(response);
    return {
      content: data.content?.[0]?.text ?? '',
      inputTokens: data.usage?.input_tokens ?? 0,
      outputTokens: data.usage?.output_tokens ?? 0,
      model: data.model ?? this.model,
    };
  }

  estimateCost(inputTokens: number, outputTokens: number): number {
    // Haiku 4.5 pricing: $0.80/MTok input, $4.00/MTok output
    return (inputTokens * 0.0000008) + (outputTokens * 0.000004);
  }

  async healthCheck(): Promise<boolean> {
    return this.apiKey.length > 0;
  }
}

/**
 * OpenAI adapter — uses Chat Completions API with gpt-4o-mini for cheapest assessments.
 * Reads OPENAI_API_KEY from environment.
 */
export class OpenAIAdapter implements LLMAdapter {
  readonly name = 'openai';
  private readonly model: string;
  private readonly apiKey: string;

  constructor(config?: Record<string, unknown>) {
    this.model = (config?.model as string) ?? 'gpt-4o-mini';
    this.apiKey = (config?.apiKey as string) ?? process.env.OPENAI_API_KEY ?? '';
  }

  async assess(prompt: string, maxTokens: number): Promise<LLMResponse> {
    const body = JSON.stringify({
      model: this.model,
      max_tokens: maxTokens,
      messages: [{ role: 'user', content: prompt }],
    });

    const response = await httpPost('api.openai.com', '/v1/chat/completions', body, {
      Authorization: `Bearer ${this.apiKey}`,
      'Content-Type': 'application/json',
    });

    const data = JSON.parse(response);
    return {
      content: data.choices?.[0]?.message?.content ?? '',
      inputTokens: data.usage?.prompt_tokens ?? 0,
      outputTokens: data.usage?.completion_tokens ?? 0,
      model: data.model ?? this.model,
    };
  }

  estimateCost(inputTokens: number, outputTokens: number): number {
    // gpt-4o-mini pricing: $0.15/MTok input, $0.60/MTok output
    return (inputTokens * 0.00000015) + (outputTokens * 0.0000006);
  }

  async healthCheck(): Promise<boolean> {
    return this.apiKey.length > 0;
  }
}

/**
 * Ollama adapter — uses local Ollama server for zero-cost assessments.
 * No API key needed. Requires Ollama running locally.
 */
export class OllamaAdapter implements LLMAdapter {
  readonly name = 'ollama';
  private readonly model: string;
  private readonly host: string;

  constructor(config?: Record<string, unknown>) {
    this.model = (config?.model as string) ?? 'llama3.2:1b';
    this.host = (config?.host as string) ?? 'http://localhost:11434';
  }

  async assess(prompt: string, maxTokens: number): Promise<LLMResponse> {
    const url = new URL(this.host);
    const body = JSON.stringify({
      model: this.model,
      prompt,
      stream: false,
      options: { num_predict: maxTokens },
    });

    const response = await httpPost(
      url.hostname,
      '/api/generate',
      body,
      { 'Content-Type': 'application/json' },
      url.port ? parseInt(url.port) : 11434,
      url.protocol === 'http:',
    );

    const data = JSON.parse(response);
    return {
      content: data.response ?? '',
      inputTokens: data.prompt_eval_count ?? 0,
      outputTokens: data.eval_count ?? 0,
      model: this.model,
    };
  }

  estimateCost(): number {
    return 0; // Local, free
  }

  async healthCheck(): Promise<boolean> {
    try {
      const url = new URL(this.host);
      await httpGet(url.hostname, '/api/tags', url.port ? parseInt(url.port) : 11434, url.protocol === 'http:');
      return true;
    } catch {
      return false;
    }
  }
}

/** Create an adapter from config */
export function createAdapter(type: LLMAdapterType, config?: Record<string, unknown>): LLMAdapter {
  switch (type) {
    case 'anthropic': return new AnthropicAdapter(config);
    case 'openai': return new OpenAIAdapter(config);
    case 'ollama': return new OllamaAdapter(config);
    case 'agent-proxy':
      // Auto-detect: try Anthropic first, then OpenAI, then Ollama
      if (process.env.ANTHROPIC_API_KEY) return new AnthropicAdapter(config);
      if (process.env.OPENAI_API_KEY) return new OpenAIAdapter(config);
      return new OllamaAdapter(config);
    default:
      throw new Error(`Unknown LLM adapter type: ${type}`);
  }
}

/** Auto-detect the best available adapter */
export function autoDetectAdapter(config?: Record<string, unknown>): LLMAdapter {
  return createAdapter('agent-proxy', config);
}

// --- HTTP helpers (zero-dependency) ---

function httpPost(
  host: string, path: string, body: string,
  headers: Record<string, string>,
  port?: number, useHttp?: boolean,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const mod = useHttp ? nodeRequire('http') : https;
    const options = {
      hostname: host,
      port: port ?? (useHttp ? 80 : 443),
      path,
      method: 'POST',
      headers: { ...headers, 'Content-Length': Buffer.byteLength(body) },
      timeout: 30000,
    };

    const req = mod.request(options, (res: import('http').IncomingMessage) => {
      let data = '';
      res.on('data', (chunk: Buffer) => { data += chunk.toString(); });
      res.on('end', () => {
        if (res.statusCode && res.statusCode >= 400) {
          reject(new Error(`HTTP ${res.statusCode}: ${data.slice(0, 200)}`));
        } else {
          resolve(data);
        }
      });
    });

    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Request timeout')); });
    req.write(body);
    req.end();
  });
}

function httpGet(
  host: string, path: string, port?: number, useHttp?: boolean,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const mod = useHttp ? nodeRequire('http') : https;
    const options = { hostname: host, port: port ?? (useHttp ? 80 : 443), path, timeout: 5000 };

    const req = mod.request(options, (res: import('http').IncomingMessage) => {
      let data = '';
      res.on('data', (chunk: Buffer) => { data += chunk.toString(); });
      res.on('end', () => resolve(data));
    });

    req.on('error', reject);
    req.on('timeout', () => { req.destroy(); reject(new Error('Request timeout')); });
    req.end();
  });
}
