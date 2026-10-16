/**
 * The README's "Default rules" table under Runtime Protection lists the rules
 * the ARP event engine applies when its config has no `rules` field, and
 * nothing else: each row's name, condition, action and `requireLlmConfirmation`
 * value is the engine's, in the engine's order.
 *
 * `defaultRules()` is private to event-engine.ts, so this reads the list a
 * default-configured `EventEngine` holds (`new EventEngine(config)` without
 * `rules` keeps `defaultRules()` as its rule list). The condition column is
 * written as the object literal a caller would put in `rules`, and is compared
 * after rendering the engine's condition the same way.
 */

import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import { EventEngine } from './arp/engine/event-engine';
import { defaultConfig } from './arp/config/loader';
import type { AlertRule } from './arp/types';

const readme = readFileSync(join(__dirname, '..', 'README.md'), 'utf-8');

function defaultRuleSection(text: string = readme): string {
  const start = text.indexOf('\n### Default rules\n');
  if (start === -1) return '';
  // The section's body runs from the line after its heading to the next heading.
  const body = text.slice(text.indexOf('\n', start + 1) + 1);
  const next = body.search(/^##+ /m);
  return next === -1 ? body : body.slice(0, next);
}

/** The table's rows whose first cell is a code span, as trimmed cells. */
function tableRows(section: string): string[][] {
  return section
    .split('\n')
    .filter((line) => /^\|\s*`[^`]+`\s*\|/.test(line))
    .map((line) =>
      line
        .slice(1, line.lastIndexOf('|'))
        .split('|')
        .map((cell) => cell.trim()),
    );
}

/** Render a rule condition as the object literal the README's table shows. */
function literal(value: unknown): string {
  if (typeof value === 'string') return `'${value}'`;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
    const entries = Object.entries(value).map(
      ([key, v]) => `${/^[A-Za-z_$][\w$]*$/.test(key) ? key : `'${key}'`}: ${literal(v)}`,
    );
    return `{ ${entries.join(', ')} }`;
  }
  throw new Error(`the README table has no rendering for ${JSON.stringify(value)}`);
}

function rulesOf(engine: EventEngine): AlertRule[] {
  return (engine as unknown as { rules: AlertRule[] }).rules;
}

describe('README "Default rules" table matches the event engine', () => {
  const defaults = rulesOf(new EventEngine({ agentName: 'readme-default-rules' }));
  const expected = defaults.map((rule) => [
    `\`${rule.name}\``,
    `\`${literal(rule.condition)}\``,
    `\`${rule.action}\``,
    rule.requireLlmConfirmation ? '`true`' : '',
  ]);

  it('the engine has default rules to compare against', () => {
    expect(Array.isArray(defaults)).toBe(true);
    expect(defaults.length).toBeGreaterThan(0);
  });

  it('lists every default rule, and no other, with its condition, action and flag', () => {
    const section = defaultRuleSection();
    expect(section, 'README.md has no "### Default rules" section').not.toBe('');
    expect(tableRows(section)).toEqual(expected);
  });

  it('a table with a rule the engine lacks, or another action, does not match', () => {
    const section = defaultRuleSection();
    const extraRow =
      "| `extra-rule` | `{ category: 'threat' }` | `alert` | |\n";
    const withExtra = section.replace(/^\| `critical-threat`/m, `${extraRow}| \`critical-threat\``);
    expect(tableRows(withExtra)).toHaveLength(expected.length + 1);
    expect(tableRows(withExtra)).not.toEqual(expected);

    const otherAction = section.replace(/^(\| `critical-threat` \|[^|]*\| )`kill`/m, '$1`alert`');
    expect(otherAction).not.toBe(section);
    expect(tableRows(otherAction)).not.toEqual(expected);
  });

  it('says the defaults are replaced, not merged, by `rules`', () => {
    const section = defaultRuleSection();
    expect(section).toMatch(/`rules`[^.]*replaces/);
    // `rules` present but empty means no rule at all, not the defaults.
    expect(rulesOf(new EventEngine({ agentName: 'readme-default-rules', rules: [] }))).toEqual([]);
  });

  it('says a config from loadConfig carries an empty rule list', () => {
    // loadConfig() starts from defaultConfig(), whose `rules` is an empty list,
    // so ARP built with no argument or from a config file has no rules unless
    // the file lists some. The README states this; if defaultConfig() ever
    // carries rules, the README sentence has to change with it.
    expect(defaultConfig().rules).toEqual([]);
    expect(rulesOf(new EventEngine(defaultConfig()))).toEqual([]);
    expect(defaultRuleSection()).toMatch(/`loadConfig\(\)`[^.]*`rules: \[\]`/);
  });
});
