/**
 * Capability Auto-Detection for JavaScript/TypeScript Agents
 *
 * This module automatically detects agent capabilities through:
 * 1. Package.json dependency analysis (npm packages)
 * 2. Configuration file (.aim/capabilities.json)
 * 3. Runtime environment detection
 */

import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

/**
 * CapabilitiesConfig represents the .aim/capabilities.json config file
 */
export interface CapabilitiesConfig {
  capabilities: string[];
  lastUpdated: string;
  version: string;
}

/**
 * CapabilityDetectionResult contains detected capabilities and metadata
 */
export interface CapabilityDetectionResult {
  capabilities: string[];
  detectedFrom: string[]; // "package.json", "config", "runtime"
  metadata: Record<string, string>;
}

/**
 * CapabilityDetector auto-detects agent capabilities
 */
export class CapabilityDetector {
  /**
   * Map npm packages to capabilities
   */
  private packageToCapability: Record<string, string> = {
    // File System
    'fs': 'read_files',
    'fs-extra': 'read_files',
    'graceful-fs': 'read_files',

    // Database
    'pg': 'access_database',           // PostgreSQL
    'mysql': 'access_database',        // MySQL
    'mysql2': 'access_database',       // MySQL
    'mongodb': 'access_database',      // MongoDB
    'mongoose': 'access_database',     // MongoDB ORM
    'sequelize': 'access_database',    // SQL ORM
    'typeorm': 'access_database',      // TypeScript ORM
    'prisma': 'access_database',       // Prisma ORM
    'knex': 'access_database',         // SQL query builder

    // HTTP/API
    'axios': 'make_api_calls',
    'node-fetch': 'make_api_calls',
    'got': 'make_api_calls',
    'request': 'make_api_calls',
    'superagent': 'make_api_calls',
    'http': 'make_api_calls',
    'https': 'make_api_calls',
    'express': 'make_api_calls',
    'koa': 'make_api_calls',
    'fastify': 'make_api_calls',

    // Code Execution
    'child_process': 'execute_code',
    'shelljs': 'execute_code',
    'execa': 'execute_code',

    // Cloud Services
    'aws-sdk': 'access_cloud_services',         // AWS SDK v2
    '@aws-sdk/client-s3': 'access_cloud_services', // AWS SDK v3
    '@google-cloud/storage': 'access_cloud_services', // Google Cloud
    '@azure/storage-blob': 'access_cloud_services',   // Azure

    // Web Scraping
    'cheerio': 'web_scraping',
    'puppeteer': 'web_automation',
    'playwright': 'web_automation',
    'jsdom': 'web_scraping',

    // Data Processing
    'csv-parser': 'data_processing',
    'xml2js': 'data_processing',
    'yaml': 'data_processing',
    'json5': 'data_processing',

    // AI/ML
    'openai': 'ai_model_access',
    '@anthropic-ai/sdk': 'ai_model_access',
    'langchain': 'ai_model_access',
    '@langchain/core': 'ai_model_access',

    // Email
    'nodemailer': 'send_email',
    'mailgun-js': 'send_email',
    'sendgrid': 'send_email',
  };

  /**
   * Run all detection methods and return combined unique capabilities
   */
  async detectAll(): Promise<CapabilityDetectionResult> {
    const capabilitiesSet = new Set<string>();
    const detectedFrom: string[] = [];
    const metadata: Record<string, string> = {};

    // 1. Detect from package.json
    try {
      const pkgCaps = await this.detectFromPackageJson();
      if (pkgCaps.length > 0) {
        pkgCaps.forEach(cap => capabilitiesSet.add(cap));
        detectedFrom.push('package.json');
      }
    } catch (err) {
      // package.json not found or unreadable
    }

    // 2. Detect from config file
    try {
      const configCaps = await this.detectFromConfig();
      if (configCaps.length > 0) {
        configCaps.forEach(cap => capabilitiesSet.add(cap));
        detectedFrom.push('config');
      }
    } catch (err) {
      // Config not found or unreadable
    }

    // 3. Detect from runtime environment
    const runtimeCaps = this.detectFromRuntime();
    if (runtimeCaps.length > 0) {
      runtimeCaps.forEach(cap => capabilitiesSet.add(cap));
      detectedFrom.push('runtime');
    }

    // Add metadata
    metadata['node_version'] = process.version;
    metadata['platform'] = process.platform;
    metadata['arch'] = process.arch;

    return {
      capabilities: Array.from(capabilitiesSet).sort(),
      detectedFrom,
      metadata
    };
  }

  /**
   * Detect capabilities from package.json file
   */
  async detectFromPackageJson(): Promise<string[]> {
    const capabilities = new Set<string>();

    // Find package.json file
    const packageJsonPath = await this.findPackageJson();
    if (!packageJsonPath) {
      return [];
    }

    // Read package.json
    const content = await fs.promises.readFile(packageJsonPath, 'utf-8');
    const packageJson = JSON.parse(content);

    // Check both dependencies and devDependencies
    const allDeps = {
      ...packageJson.dependencies,
      ...packageJson.devDependencies
    };

    // Map package names to capabilities
    for (const pkgName in allDeps) {
      if (pkgName in this.packageToCapability) {
        capabilities.add(this.packageToCapability[pkgName]);
      }
    }

    return Array.from(capabilities);
  }

  /**
   * Detect capabilities from .aim/capabilities.json config file
   */
  async detectFromConfig(): Promise<string[]> {
    const configPath = this.getCapabilitiesConfigPath();
    if (!configPath) {
      return [];
    }

    try {
      const content = await fs.promises.readFile(configPath, 'utf-8');
      const config: CapabilitiesConfig = JSON.parse(content);
      return config.capabilities || [];
    } catch (err) {
      return [];
    }
  }

  /**
   * Detect capabilities from runtime environment
   */
  detectFromRuntime(): string[] {
    const capabilities: string[] = [];

    // Check if running with elevated permissions (sudo/admin)
    if (this.hasElevatedPermissions()) {
      capabilities.push('elevated_permissions');
    }

    // Check network access (always available in Node.js)
    capabilities.push('network_access');

    return capabilities;
  }

  /**
   * Find package.json file in current directory and parents
   */
  private async findPackageJson(): Promise<string | null> {
    let currentDir = process.cwd();

    while (true) {
      const packageJsonPath = path.join(currentDir, 'package.json');

      try {
        await fs.promises.access(packageJsonPath, fs.constants.R_OK);
        return packageJsonPath;
      } catch (err) {
        // Not found, try parent directory
      }

      const parentDir = path.dirname(currentDir);
      if (parentDir === currentDir) {
        // Reached root directory
        break;
      }
      currentDir = parentDir;
    }

    return null;
  }

  /**
   * Get path to .aim/capabilities.json config file
   */
  private getCapabilitiesConfigPath(): string | null {
    // Check project-local config first
    const localConfig = path.join(process.cwd(), '.aim', 'capabilities.json');
    if (fs.existsSync(localConfig)) {
      return localConfig;
    }

    // Check home directory config
    const homeDir = os.homedir();
    const homeConfig = path.join(homeDir, '.aim', 'capabilities.json');
    if (fs.existsSync(homeConfig)) {
      return homeConfig;
    }

    return null;
  }

  /**
   * Check if running with elevated permissions
   */
  private hasElevatedPermissions(): boolean {
    // On Unix-like systems, check if effective UID is 0 (root)
    if (process.platform !== 'win32') {
      return process.getuid ? process.getuid() === 0 : false;
    }
    // On Windows, this would require more complex checks
    return false;
  }
}

/**
 * Auto-detect capabilities (convenience function)
 */
export async function autoDetectCapabilities(): Promise<string[]> {
  const detector = new CapabilityDetector();
  const result = await detector.detectAll();
  return result.capabilities;
}

/**
 * Save capabilities to .aim/capabilities.json
 */
export async function saveCapabilitiesConfig(capabilities: string[]): Promise<void> {
  const homeDir = os.homedir();
  const aimDir = path.join(homeDir, '.aim');

  // Create .aim directory
  await fs.promises.mkdir(aimDir, { recursive: true, mode: 0o700 });

  // Create config
  const config: CapabilitiesConfig = {
    capabilities,
    lastUpdated: new Date().toISOString(),
    version: '1.0.0'
  };

  // Write to file
  const configPath = path.join(aimDir, 'capabilities.json');
  await fs.promises.writeFile(
    configPath,
    JSON.stringify(config, null, 2),
    { mode: 0o600 }
  );
}
