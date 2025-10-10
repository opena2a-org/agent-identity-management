/**
 * Auto-detect agent capabilities from imports
 * Similar to Python SDK capability detection
 */
export function autoDetectCapabilities(): string[] {
  const capabilities = new Set<string>();

  // Common package to capability mappings
  const packageMappings: Record<string, string> = {
    'axios': 'make_api_calls',
    'node-fetch': 'make_api_calls',
    'fetch': 'make_api_calls',
    'nodemailer': 'send_email',
    'pg': 'access_database',
    'mysql': 'access_database',
    'mysql2': 'access_database',
    'mongodb': 'access_database',
    'fs': 'read_files',
    'fs/promises': 'read_files',
    'child_process': 'execute_code',
    '@anthropic-ai/sdk': 'use_ai_models',
    'openai': 'use_ai_models',
    '@google-ai/generativelanguage': 'use_ai_models',
  };

  // Check require.cache for loaded modules
  if (typeof require !== 'undefined' && require.cache) {
    const loadedModules = Object.keys(require.cache);

    loadedModules.forEach(modulePath => {
      Object.keys(packageMappings).forEach(packageName => {
        if (modulePath.includes(`/node_modules/${packageName}/`)) {
          capabilities.add(packageMappings[packageName]);
        }
      });
    });
  }

  // Always include basic capabilities
  capabilities.add('read_files');
  capabilities.add('write_files');

  return Array.from(capabilities).sort();
}
