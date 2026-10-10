import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The repository README, the npm page and the PyPI page link to the project
 * page on opena2a.org.
 *
 * These three pages are where most readers meet the platform first. The npm
 * "Homepage" link pointed back at the repository README, the PyPI "Homepage"
 * link at the repository, and the first link in each README at a status file,
 * a PyPI badge or the hosted sign-up page. None of them reached
 * https://opena2a.org/agent-identity-management, the page that describes the
 * platform (https://opena2a.org/aim declares it as its canonical URL).
 *
 * These cells run in the SDK suite because that job runs on every pull
 * request, so an edit that points a surface elsewhere fails before merge.
 *
 *   C1  sdk/typescript/package.json `homepage` (the npm "Homepage" link) is
 *       the project page;
 *   C2  sdk/python/setup.py `url` (the PyPI "Homepage" link) is the project
 *       page, and no `project_urls` entry named Homepage overrides it;
 *   C3  the first link in README.md (GitHub), sdk/python/README.md (the PyPI
 *       description) and sdk/typescript/README.md (the npm readme) is the
 *       project page;
 *   C4  the pre-fix values and near misses, kept as literals, are refused,
 *       and the accepted forms pass, through the same predicates as C1-C3.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const read = (relative: string): string =>
  readFileSync(join(REPO_ROOT, relative), 'utf-8');

const PROJECT_PAGE = 'https://opena2a.org/agent-identity-management';

const READMES = ['README.md', 'sdk/python/README.md', 'sdk/typescript/README.md'];

function isProjectPage(url: string | null | undefined): boolean {
  return url === PROJECT_PAGE;
}

/**
 * The target of the first link a reader can follow, in document order.
 * Image sources are not links (a badge's link is the target around the
 * image), and fenced code blocks hold no links.
 */
function firstLinkTarget(markdown: string): string | null {
  const text = markdown
    .replace(/^(```|~~~)[^\n]*\n[\s\S]*?^\1[^\n]*$/gm, '')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, 'IMAGE')
    .replace(/<img\b[^>]*>/gi, 'IMAGE');
  const patterns = [
    /\[[^\]]*\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)/,
    /<a\b[^>]*\bhref\s*=\s*["']([^"']+)["']/i,
    /<(https?:\/\/[^>\s]+)>/,
  ];
  let first: { index: number; target: string } | null = null;
  for (const pattern of patterns) {
    const match = pattern.exec(text);
    if (match && (first === null || match.index < first.index)) {
      first = { index: match.index, target: match[1] };
    }
  }
  return first === null ? null : first.target;
}

function setupPyHomepage(source: string): string | null {
  const match = /^\s*url\s*=\s*["']([^"']+)["']/m.exec(source);
  return match === null ? null : match[1];
}

function setupPyHomepageOverride(source: string): string | null {
  const match = /["']Homepage["']\s*:\s*["']([^"']+)["']/i.exec(source);
  return match === null ? null : match[1];
}

describe('project page links on the README, npm and PyPI pages', () => {
  it('C1: the npm homepage is the project page', () => {
    const pkg = JSON.parse(read('sdk/typescript/package.json')) as { homepage?: string };
    expect(pkg.homepage, 'sdk/typescript/package.json homepage').toBe(PROJECT_PAGE);
  });

  it('C2: the PyPI homepage is the project page', () => {
    const setup = read('sdk/python/setup.py');
    expect(setupPyHomepage(setup), 'sdk/python/setup.py url=').toBe(PROJECT_PAGE);
    const override = setupPyHomepageOverride(setup);
    expect(
      override === null || isProjectPage(override),
      `sdk/python/setup.py project_urls Homepage is ${override}`,
    ).toBe(true);
  });

  for (const readme of READMES) {
    it(`C3: the first link in ${readme} is the project page`, () => {
      expect(firstLinkTarget(read(readme)), `${readme} first link`).toBe(PROJECT_PAGE);
    });
  }

  it('C4: the pre-fix values and near misses are refused', () => {
    const refusedUrls = [
      'https://github.com/opena2a-org/agent-identity-management#readme',
      'https://github.com/opena2a-org/agent-identity-management',
      'https://opena2a.org',
      'https://opena2a.org/',
      'https://opena2a.org/aim',
      'https://opena2a.org/docs/aim',
      'http://opena2a.org/agent-identity-management',
      'https://aim.opena2a.org/get-started',
    ];
    for (const url of refusedUrls) {
      expect(isProjectPage(url), url).toBe(false);
    }

    const refusedReadmes = [
      // Root README before the fix: a status badge links to STATUS.md.
      '# OpenA2A AIM (Agent Identity Management)\n\n' +
        '[![Status: stable](https://img.shields.io/badge/status-stable-brightgreen)](./STATUS.md)\n\n' +
        `See [the project page](${PROJECT_PAGE}).\n`,
      // PyPI description before the fix: a version badge links to PyPI.
      '# AIM Python SDK\n\n[![PyPI version](https://img.shields.io/pypi/v/aim-sdk.svg)](https://pypi.org/project/aim-sdk/)\n',
      // npm readme before the fix: the hosted sign-up page.
      '# AIM SDK for TypeScript/Node.js\n\nManaged hosting at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started).\n',
      // An image whose source is the page is not a link to it.
      `# AIM\n\n![logo](${PROJECT_PAGE})\n\n[repo](https://github.com/opena2a-org/agent-identity-management)\n`,
      // An HTML anchor ahead of the markdown link is the first link.
      `<a href="https://opena2a.org/">OpenA2A</a>\n\n# [AIM](${PROJECT_PAGE})\n`,
      // No link at all.
      '# AIM\n\nNo links here.\n',
    ];
    for (const text of refusedReadmes) {
      expect(isProjectPage(firstLinkTarget(text)), JSON.stringify(text)).toBe(false);
    }

    const refusedSetups = [
      'setup(\n    url="https://github.com/opena2a-org/agent-identity-management",\n)\n',
      `setup(\n    url="${PROJECT_PAGE}",\n    project_urls={"Homepage": "https://opena2a.org"},\n)\n`,
    ];
    for (const source of refusedSetups) {
      const override = setupPyHomepageOverride(source);
      const accepted =
        isProjectPage(setupPyHomepage(source)) && (override === null || isProjectPage(override));
      expect(accepted, JSON.stringify(source)).toBe(false);
    }
  });

  it('C4: the accepted forms pass', () => {
    const acceptedReadmes = [
      `# [OpenA2A AIM (Agent Identity Management)](${PROJECT_PAGE})\n\n[![Status](https://img.shields.io/badge/x)](./STATUS.md)\n`,
      '```bash\ncurl [x](https://example.com)\n```\n\n' + `See [the project page](<${PROJECT_PAGE}>).\n`,
      `Read more at <${PROJECT_PAGE}>, or [the repository](https://github.com/opena2a-org/agent-identity-management).\n`,
    ];
    for (const text of acceptedReadmes) {
      expect(isProjectPage(firstLinkTarget(text)), JSON.stringify(text)).toBe(true);
    }

    const acceptedSetup = `setup(\n    url="${PROJECT_PAGE}",\n    project_urls={"Source": "https://github.com/opena2a-org/agent-identity-management"},\n)\n`;
    expect(setupPyHomepage(acceptedSetup)).toBe(PROJECT_PAGE);
    expect(setupPyHomepageOverride(acceptedSetup)).toBeNull();
  });
});
