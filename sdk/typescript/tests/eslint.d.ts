// ESLint 8 ships no type declarations. This declares the part of its API that
// tests/lint-script-coverage.test.ts uses.
declare module 'eslint' {
  export class ESLint {
    constructor(options?: { cwd?: string });
    isPathIgnored(filePath: string): Promise<boolean>;
  }
}
