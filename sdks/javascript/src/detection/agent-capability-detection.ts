/**
 * Agent Capability Detection
 *
 * Detects what an agent CAN DO (beyond just MCPs):
 * - Programming environment (language, frameworks)
 * - AI model usage
 * - File system operations
 * - Database operations
 * - Network operations
 * - Code execution capabilities
 * - Risk scoring
 */

import * as fs from 'fs';
import * as path from 'path';

// ============================================================================
// TYPES
// ============================================================================

export type RiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';

export interface ProgrammingEnvironment {
  language: string;
  runtime: string;
  version: string;
  frameworks: string[];
  packageManager: 'npm' | 'yarn' | 'pnpm' | 'bun' | 'pip' | 'poetry' | 'cargo' | 'go' | 'unknown';
}

export interface AIModelUsage {
  provider: string;
  models: string[];
  usage: 'primary' | 'fallback' | 'experimental';
  batchAPI: boolean;
}

export interface FileSystemCapability {
  read: boolean;
  write: boolean;
  delete: boolean;
  paths: string[];
  riskLevel: RiskLevel;
  riskScore: number; // 0-100
}

export interface DatabaseCapability {
  types: string[];
  operations: ('read' | 'write' | 'delete' | 'schema')[];
  riskLevel: RiskLevel;
  riskScore: number;
}

export interface NetworkCapability {
  http: boolean;
  websocket: boolean;
  tcp: boolean;
  externalAPIs: string[];
  riskLevel: RiskLevel;
  riskScore: number;
}

export interface CodeExecutionCapability {
  eval: boolean;
  exec: boolean;
  shellCommands: boolean;
  childProcesses: boolean;
  riskLevel: RiskLevel;
  riskScore: number;
}

export interface CredentialAccessCapability {
  keyring: boolean;
  envVars: boolean;
  configFiles: boolean;
  riskLevel: RiskLevel;
  riskScore: number;
}

export interface BrowserAutomationCapability {
  puppeteer: boolean;
  playwright: boolean;
  selenium: boolean;
  riskLevel: RiskLevel;
  riskScore: number;
}

export interface SecurityAlert {
  severity: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  capability: string;
  message: string;
  recommendation: string;
  trustScoreImpact: number; // negative number
}

export interface RiskAssessment {
  overallRiskScore: number; // 0-100 (higher = riskier)
  riskLevel: RiskLevel;
  trustScoreImpact: number; // -50 to 0
  alerts: SecurityAlert[];
}

export interface AgentCapabilityDetectionResult {
  detectedAt: string;
  environment: ProgrammingEnvironment;
  aiModels: AIModelUsage[];
  capabilities: {
    fileSystem?: FileSystemCapability;
    database?: DatabaseCapability;
    network?: NetworkCapability;
    codeExecution?: CodeExecutionCapability;
    credentialAccess?: CredentialAccessCapability;
    browserAutomation?: BrowserAutomationCapability;
  };
  riskAssessment: RiskAssessment;
  detectionConfig: AgentCapabilityDetectionConfig;
}

export interface AgentCapabilityDetectionConfig {
  scanPackages?: boolean;
  scanImports?: boolean;
  scanSourceCode?: boolean;
  detectAIModels?: boolean;
  detectFileSystem?: boolean;
  detectDatabase?: boolean;
  detectNetwork?: boolean;
  detectCodeExecution?: boolean;
  detectCredentials?: boolean;
  detectBrowserAutomation?: boolean;
}

// ============================================================================
// DEFAULT CONFIGURATION
// ============================================================================

function getDefaultConfig(): Required<AgentCapabilityDetectionConfig> {
  return {
    scanPackages: true,
    scanImports: true,
    scanSourceCode: true,
    detectAIModels: true,
    detectFileSystem: true,
    detectDatabase: true,
    detectNetwork: true,
    detectCodeExecution: true,
    detectCredentials: true,
    detectBrowserAutomation: true,
  };
}

// ============================================================================
// MAIN DETECTION FUNCTION
// ============================================================================

export async function detectAgentCapabilities(
  config?: AgentCapabilityDetectionConfig
): Promise<AgentCapabilityDetectionResult> {
  const cfg = { ...getDefaultConfig(), ...config };

  // Detect programming environment
  const environment = await detectProgrammingEnvironment();

  // Detect AI model usage
  const aiModels = cfg.detectAIModels ? await detectAIModelUsage() : [];

  // Detect capabilities
  const capabilities: AgentCapabilityDetectionResult['capabilities'] = {};

  if (cfg.detectFileSystem) {
    capabilities.fileSystem = await detectFileSystemCapability();
  }

  if (cfg.detectDatabase) {
    capabilities.database = await detectDatabaseCapability();
  }

  if (cfg.detectNetwork) {
    capabilities.network = await detectNetworkCapability();
  }

  if (cfg.detectCodeExecution) {
    capabilities.codeExecution = await detectCodeExecutionCapability();
  }

  if (cfg.detectCredentials) {
    capabilities.credentialAccess = await detectCredentialAccessCapability();
  }

  if (cfg.detectBrowserAutomation) {
    capabilities.browserAutomation = await detectBrowserAutomationCapability();
  }

  // Calculate risk assessment
  const riskAssessment = calculateRiskAssessment(capabilities);

  return {
    detectedAt: new Date().toISOString(),
    environment,
    aiModels,
    capabilities,
    riskAssessment,
    detectionConfig: cfg,
  };
}

// ============================================================================
// ENVIRONMENT DETECTION
// ============================================================================

async function detectProgrammingEnvironment(): Promise<ProgrammingEnvironment> {
  const packageJsonPath = path.join(process.cwd(), 'package.json');
  let frameworks: string[] = [];
  let packageManager: ProgrammingEnvironment['packageManager'] = 'npm';

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      // Detect frameworks
      const frameworkPatterns = {
        'react': ['react'],
        'next': ['next'],
        'express': ['express'],
        'fastify': ['fastify'],
        'nestjs': ['@nestjs/core'],
        'langchain': ['langchain', '@langchain/core'],
        'autogpt': ['autogpt'],
        'crewai': ['crewai'],
      };

      for (const [framework, patterns] of Object.entries(frameworkPatterns)) {
        for (const pattern of patterns) {
          if (allDeps[pattern]) {
            frameworks.push(framework);
            break;
          }
        }
      }

      // Detect package manager
      if (fs.existsSync(path.join(process.cwd(), 'yarn.lock'))) {
        packageManager = 'yarn';
      } else if (fs.existsSync(path.join(process.cwd(), 'pnpm-lock.yaml'))) {
        packageManager = 'pnpm';
      } else if (fs.existsSync(path.join(process.cwd(), 'bun.lockb'))) {
        packageManager = 'bun';
      }
    }
  } catch (err) {
    // Continue with defaults
  }

  return {
    language: 'javascript',
    runtime: 'node',
    version: process.version,
    frameworks,
    packageManager,
  };
}

// ============================================================================
// AI MODEL DETECTION
// ============================================================================

async function detectAIModelUsage(): Promise<AIModelUsage[]> {
  const models: AIModelUsage[] = [];
  const packageJsonPath = path.join(process.cwd(), 'package.json');

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      // Anthropic Claude
      if (allDeps['@anthropic-ai/sdk']) {
        models.push({
          provider: 'anthropic',
          models: ['claude-3-opus', 'claude-3-sonnet', 'claude-3-haiku'],
          usage: 'primary',
          batchAPI: false,
        });
      }

      // OpenAI
      if (allDeps['openai']) {
        models.push({
          provider: 'openai',
          models: ['gpt-4', 'gpt-4-turbo', 'gpt-3.5-turbo'],
          usage: 'primary',
          batchAPI: false,
        });
      }

      // Google Gemini
      if (allDeps['@google/generative-ai']) {
        models.push({
          provider: 'google',
          models: ['gemini-pro', 'gemini-pro-vision'],
          usage: 'primary',
          batchAPI: false,
        });
      }

      // LangChain (could use multiple providers)
      if (allDeps['langchain'] || allDeps['@langchain/core']) {
        models.push({
          provider: 'langchain',
          models: ['multiple'],
          usage: 'primary',
          batchAPI: false,
        });
      }
    }
  } catch (err) {
    // Continue with empty models
  }

  return models;
}

// ============================================================================
// FILE SYSTEM CAPABILITY DETECTION
// ============================================================================

async function detectFileSystemCapability(): Promise<FileSystemCapability> {
  let read = false;
  let write = false;
  let del = false;
  const paths: string[] = [];

  const packageJsonPath = path.join(process.cwd(), 'package.json');

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      // Check for fs/fs-extra usage
      if (allDeps['fs-extra'] || allDeps['fs']) {
        read = true;
        write = true;
        del = true;
      }

      // Check for file-related packages
      const filePackages = ['glob', 'fast-glob', 'rimraf', 'del', 'mkdirp'];
      for (const pkg of filePackages) {
        if (allDeps[pkg]) {
          read = true;
          write = true;
          if (pkg === 'rimraf' || pkg === 'del') {
            del = true;
          }
        }
      }
    }

    // Scan source code for fs usage (basic detection)
    const srcDir = path.join(process.cwd(), 'src');
    if (fs.existsSync(srcDir)) {
      const files = fs.readdirSync(srcDir).filter((f) => f.endsWith('.ts') || f.endsWith('.js'));
      for (const file of files.slice(0, 10)) {
        // Limit to 10 files for performance
        const content = fs.readFileSync(path.join(srcDir, file), 'utf-8');
        if (content.includes('fs.read') || content.includes('readFile')) {
          read = true;
        }
        if (content.includes('fs.write') || content.includes('writeFile')) {
          write = true;
        }
        if (content.includes('fs.unlink') || content.includes('fs.rm')) {
          del = true;
        }
      }
    }
  } catch (err) {
    // Continue with defaults
  }

  // Calculate risk
  let riskScore = 0;
  if (read) riskScore += 10;
  if (write) riskScore += 25;
  if (del) riskScore += 35;

  const riskLevel: RiskLevel = riskScore >= 60 ? 'HIGH' : riskScore >= 30 ? 'MEDIUM' : 'LOW';

  return {
    read,
    write,
    delete: del,
    paths,
    riskLevel,
    riskScore,
  };
}

// ============================================================================
// DATABASE CAPABILITY DETECTION
// ============================================================================

async function detectDatabaseCapability(): Promise<DatabaseCapability> {
  const types: string[] = [];
  const operations: ('read' | 'write' | 'delete' | 'schema')[] = [];

  const packageJsonPath = path.join(process.cwd(), 'package.json');

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      // Detect database types
      const dbPatterns = {
        postgresql: ['pg', 'postgres', '@supabase/supabase-js'],
        mysql: ['mysql', 'mysql2'],
        mongodb: ['mongodb', 'mongoose'],
        sqlite: ['sqlite3', 'better-sqlite3'],
        redis: ['redis', 'ioredis'],
        qdrant: ['@qdrant/js-client-rest'],
      };

      for (const [dbType, packages] of Object.entries(dbPatterns)) {
        for (const pkg of packages) {
          if (allDeps[pkg]) {
            types.push(dbType);
            break;
          }
        }
      }

      // If database packages found, assume all operations
      if (types.length > 0) {
        operations.push('read', 'write', 'delete', 'schema');
      }
    }
  } catch (err) {
    // Continue with defaults
  }

  // Calculate risk
  let riskScore = 0;
  if (operations.includes('read')) riskScore += 15;
  if (operations.includes('write')) riskScore += 25;
  if (operations.includes('delete')) riskScore += 35;
  if (operations.includes('schema')) riskScore += 25;

  const riskLevel: RiskLevel = riskScore >= 60 ? 'HIGH' : riskScore >= 30 ? 'MEDIUM' : 'LOW';

  return {
    types,
    operations,
    riskLevel,
    riskScore,
  };
}

// ============================================================================
// NETWORK CAPABILITY DETECTION
// ============================================================================

async function detectNetworkCapability(): Promise<NetworkCapability> {
  let http = false;
  let websocket = false;
  let tcp = false;
  const externalAPIs: string[] = [];

  const packageJsonPath = path.join(process.cwd(), 'package.json');

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      // HTTP packages
      if (allDeps['axios'] || allDeps['node-fetch'] || allDeps['got'] || allDeps['request']) {
        http = true;
      }

      // WebSocket packages
      if (allDeps['ws'] || allDeps['socket.io'] || allDeps['socket.io-client']) {
        websocket = true;
      }

      // TCP packages
      if (allDeps['net']) {
        tcp = true;
      }

      // Detect known API integrations
      const apiPackages = {
        '@anthropic-ai/sdk': 'anthropic.com',
        'openai': 'openai.com',
        '@supabase/supabase-js': 'supabase.com',
        '@octokit/rest': 'github.com',
        'stripe': 'stripe.com',
      };

      for (const [pkg, api] of Object.entries(apiPackages)) {
        if (allDeps[pkg]) {
          externalAPIs.push(api);
        }
      }
    }
  } catch (err) {
    // Continue with defaults
  }

  // Calculate risk
  let riskScore = 0;
  if (http) riskScore += 15;
  if (websocket) riskScore += 20;
  if (tcp) riskScore += 30;
  riskScore += externalAPIs.length * 5;

  const riskLevel: RiskLevel = riskScore >= 60 ? 'HIGH' : riskScore >= 30 ? 'MEDIUM' : 'LOW';

  return {
    http,
    websocket,
    tcp,
    externalAPIs,
    riskLevel,
    riskScore,
  };
}

// ============================================================================
// CODE EXECUTION CAPABILITY DETECTION
// ============================================================================

async function detectCodeExecutionCapability(): Promise<CodeExecutionCapability> {
  let evalDetected = false;
  let execDetected = false;
  let shellCommands = false;
  let childProcesses = false;

  try {
    // Scan source code for dangerous patterns
    const srcDir = path.join(process.cwd(), 'src');
    if (fs.existsSync(srcDir)) {
      const files = fs.readdirSync(srcDir).filter((f) => f.endsWith('.ts') || f.endsWith('.js'));
      for (const file of files.slice(0, 20)) {
        // Limit to 20 files for performance
        const content = fs.readFileSync(path.join(srcDir, file), 'utf-8');

        // Check for eval()
        if (content.includes('eval(')) {
          evalDetected = true;
        }

        // Check for exec/spawn
        if (
          content.includes('child_process') ||
          content.includes('.exec(') ||
          content.includes('.spawn(')
        ) {
          execDetected = true;
          childProcesses = true;
        }

        // Check for shell commands
        if (content.includes('sh ') || content.includes('bash ') || content.includes('/bin/')) {
          shellCommands = true;
        }
      }
    }
  } catch (err) {
    // Continue with defaults
  }

  // Calculate risk - CODE EXECUTION IS CRITICAL RISK
  let riskScore = 0;
  if (evalDetected) riskScore += 40; // CRITICAL
  if (execDetected) riskScore += 30;
  if (shellCommands) riskScore += 20;
  if (childProcesses) riskScore += 10;

  const riskLevel: RiskLevel =
    riskScore >= 70 ? 'CRITICAL' : riskScore >= 50 ? 'HIGH' : riskScore >= 25 ? 'MEDIUM' : 'LOW';

  return {
    eval: evalDetected,
    exec: execDetected,
    shellCommands,
    childProcesses,
    riskLevel,
    riskScore,
  };
}

// ============================================================================
// CREDENTIAL ACCESS CAPABILITY DETECTION
// ============================================================================

async function detectCredentialAccessCapability(): Promise<CredentialAccessCapability> {
  let keyring = false;
  let envVars = false;
  let configFiles = false;

  const packageJsonPath = path.join(process.cwd(), 'package.json');

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      // Check for keyring packages
      if (allDeps['keytar'] || allDeps['@aim/sdk']) {
        keyring = true;
      }

      // Check for dotenv (env var access)
      if (allDeps['dotenv']) {
        envVars = true;
      }
    }

    // Scan for env var usage
    const srcDir = path.join(process.cwd(), 'src');
    if (fs.existsSync(srcDir)) {
      const files = fs.readdirSync(srcDir).filter((f) => f.endsWith('.ts') || f.endsWith('.js'));
      for (const file of files.slice(0, 10)) {
        const content = fs.readFileSync(path.join(srcDir, file), 'utf-8');
        if (content.includes('process.env')) {
          envVars = true;
        }
      }
    }

    // Check for .env file
    if (fs.existsSync(path.join(process.cwd(), '.env'))) {
      configFiles = true;
    }
  } catch (err) {
    // Continue with defaults
  }

  // Calculate risk
  let riskScore = 0;
  if (keyring) riskScore += 30;
  if (envVars) riskScore += 15;
  if (configFiles) riskScore += 10;

  const riskLevel: RiskLevel = riskScore >= 60 ? 'HIGH' : riskScore >= 30 ? 'MEDIUM' : 'LOW';

  return {
    keyring,
    envVars,
    configFiles,
    riskLevel,
    riskScore,
  };
}

// ============================================================================
// BROWSER AUTOMATION CAPABILITY DETECTION
// ============================================================================

async function detectBrowserAutomationCapability(): Promise<BrowserAutomationCapability> {
  let puppeteer = false;
  let playwright = false;
  let selenium = false;

  const packageJsonPath = path.join(process.cwd(), 'package.json');

  try {
    if (fs.existsSync(packageJsonPath)) {
      const packageJson = JSON.parse(fs.readFileSync(packageJsonPath, 'utf-8'));
      const allDeps = {
        ...packageJson.dependencies,
        ...packageJson.devDependencies,
      };

      if (allDeps['puppeteer']) puppeteer = true;
      if (allDeps['playwright']) playwright = true;
      if (allDeps['selenium-webdriver']) selenium = true;
    }
  } catch (err) {
    // Continue with defaults
  }

  // Calculate risk
  let riskScore = 0;
  if (puppeteer) riskScore += 25;
  if (playwright) riskScore += 25;
  if (selenium) riskScore += 25;

  const riskLevel: RiskLevel = riskScore >= 60 ? 'HIGH' : riskScore >= 30 ? 'MEDIUM' : 'LOW';

  return {
    puppeteer,
    playwright,
    selenium,
    riskLevel,
    riskScore,
  };
}

// ============================================================================
// RISK ASSESSMENT
// ============================================================================

function calculateRiskAssessment(
  capabilities: AgentCapabilityDetectionResult['capabilities']
): RiskAssessment {
  const alerts: SecurityAlert[] = [];
  let totalRiskScore = 0;

  // File system risks
  if (capabilities.fileSystem) {
    totalRiskScore += capabilities.fileSystem.riskScore;
    if (capabilities.fileSystem.delete) {
      alerts.push({
        severity: 'MEDIUM',
        capability: 'file_system',
        message: 'Agent can delete files from the file system',
        recommendation: 'Restrict file deletion to specific directories or require user approval',
        trustScoreImpact: -10,
      });
    }
  }

  // Database risks
  if (capabilities.database) {
    totalRiskScore += capabilities.database.riskScore;
    if (capabilities.database.operations.includes('delete')) {
      alerts.push({
        severity: 'HIGH',
        capability: 'database',
        message: 'Agent can delete data from databases',
        recommendation: 'Enable audit logging for all database deletions',
        trustScoreImpact: -15,
      });
    }
  }

  // Network risks
  if (capabilities.network) {
    totalRiskScore += capabilities.network.riskScore * 0.5; // Network is less risky
  }

  // Code execution risks - CRITICAL
  if (capabilities.codeExecution) {
    totalRiskScore += capabilities.codeExecution.riskScore;
    if (capabilities.codeExecution.eval) {
      alerts.push({
        severity: 'CRITICAL',
        capability: 'code_execution',
        message: 'Agent uses eval() - CODE INJECTION RISK',
        recommendation: 'Disable dynamic code execution or sandbox agent in isolated environment',
        trustScoreImpact: -30,
      });
    }
    if (capabilities.codeExecution.exec) {
      alerts.push({
        severity: 'HIGH',
        capability: 'code_execution',
        message: 'Agent can execute shell commands',
        recommendation: 'Require approval for all shell command executions',
        trustScoreImpact: -20,
      });
    }
  }

  // Credential access risks
  if (capabilities.credentialAccess) {
    totalRiskScore += capabilities.credentialAccess.riskScore;
    if (capabilities.credentialAccess.keyring) {
      alerts.push({
        severity: 'HIGH',
        capability: 'credential_access',
        message: 'Agent accesses system keyring',
        recommendation: 'Restrict keyring access or require user approval',
        trustScoreImpact: -15,
      });
    }
  }

  // Browser automation risks
  if (capabilities.browserAutomation) {
    totalRiskScore += capabilities.browserAutomation.riskScore * 0.7; // Less risky
  }

  // Normalize to 0-100
  const overallRiskScore = Math.min(100, totalRiskScore);

  // Determine risk level
  const riskLevel: RiskLevel =
    overallRiskScore >= 70
      ? 'CRITICAL'
      : overallRiskScore >= 50
      ? 'HIGH'
      : overallRiskScore >= 25
      ? 'MEDIUM'
      : 'LOW';

  // Calculate trust score impact (-50 to 0)
  const trustScoreImpact = Math.max(-50, Math.floor(-overallRiskScore / 2));

  return {
    overallRiskScore,
    riskLevel,
    trustScoreImpact,
    alerts,
  };
}

// ============================================================================
// EXPORTS
// ============================================================================

export default detectAgentCapabilities;
