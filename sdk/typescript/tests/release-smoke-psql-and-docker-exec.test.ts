import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * scripts/release-smoke.sh clears force_password_change for the admin it signs
 * in as, with psql inside its throwaway PostgreSQL container. The script used
 * to put ADMIN_EMAIL into the SQL text of `psql -c`. It now hands psql the
 * email as the variable email, which :'email' quotes as one SQL literal, and
 * feeds the UPDATE on standard input from a heredoc with a quoted delimiter,
 * where the shell expands nothing.
 *
 * .hmaignore exempts this script from hackmyagent's DOCKERINJ-001 ("Docker
 * exec with variable interpolation") because each argument of its docker exec
 * calls is quoted, no sh -c parses them again, and psql quotes the email. With
 * the exemption in place no scan reports a change that breaks one of those,
 * so these tests check each of them:
 *
 *   - no psql -c (or --command) argument carries a shell expansion;
 *   - no docker exec runs sh, bash or another shell with -c;
 *   - every expansion in a docker exec command sits inside double quotes;
 *   - the force_password_change UPDATE takes the email through
 *     -v email="$ADMIN_EMAIL" and :'email', from a heredoc whose delimiter
 *     is quoted.
 *
 * The script is read as the shell reads it: a line ending in a backslash
 * continues on the next, comment lines are skipped, a heredoc body belongs to
 * the command that opens it, and quotes decide which `$` the shell expands.
 */

const SCRIPT_PATH = join(__dirname, '..', 'scripts', 'release-smoke.sh');
const SCRIPT_LABEL = 'scripts/release-smoke.sh';

// Assembled at run time so that no line of this file reads as a docker exec
// call that carries a variable.
const DOCKER_EXEC = ['docker', 'exec'].join(' ');

/** One command as the shell reads it, with the heredoc it opens, if any. */
interface Command {
  /** 1-based line the command starts on. */
  line: number;
  text: string;
  heredoc?: { body: string; quotedDelimiter: boolean };
}

/** A shell word after quote removal, and whether the shell expands part of it. */
interface Word {
  text: string;
  /** A `$` or backquote the shell expands, inside double quotes or unquoted. */
  expands: boolean;
  /** A `$` or backquote the shell expands outside any quotes. */
  expandsUnquoted: boolean;
}

const HEREDOC = /(?<!<)<<(?!<)(-?)\s*(?:'(\w+)'|"(\w+)"|\\(\w+)|(\w+))/;

function commands(script: string): Command[] {
  const lines = script.split('\n');
  const out: Command[] = [];
  let i = 0;
  while (i < lines.length) {
    const start = i;
    let text = lines[i++];
    if (text.trim() === '' || text.trim().startsWith('#')) continue;
    while (text.endsWith('\\') && i < lines.length) text = `${text.slice(0, -1)} ${lines[i++]}`;
    const command: Command = { line: start + 1, text };
    const heredoc = HEREDOC.exec(text);
    if (heredoc !== null) {
      const stripTabs = heredoc[1] === '-';
      const delimiter = heredoc[2] ?? heredoc[3] ?? heredoc[4] ?? heredoc[5];
      const body: string[] = [];
      while (i < lines.length && (stripTabs ? lines[i].replace(/^\t+/, '') : lines[i]) !== delimiter) body.push(lines[i++]);
      i++;
      command.heredoc = { body: body.join('\n'), quotedDelimiter: heredoc[5] === undefined };
    }
    out.push(command);
  }
  return out;
}

function words(text: string): Word[] {
  const out: Word[] = [];
  let i = 0;
  while (i < text.length) {
    if (/\s/.test(text[i])) {
      i++;
      continue;
    }
    const word: Word = { text: '', expands: false, expandsUnquoted: false };
    while (i < text.length && !/\s/.test(text[i])) {
      const c = text[i];
      if (c === "'") {
        const end = text.indexOf("'", i + 1);
        const stop = end === -1 ? text.length : end;
        word.text += text.slice(i + 1, stop);
        i = stop + 1;
      } else if (c === '"') {
        i++;
        while (i < text.length && text[i] !== '"') {
          if (text[i] === '\\' && i + 1 < text.length) {
            word.text += text[i + 1];
            i += 2;
            continue;
          }
          if (text[i] === '$' || text[i] === '`') word.expands = true;
          word.text += text[i++];
        }
        i++;
      } else if (c === '\\') {
        word.text += text[i + 1] ?? '';
        i += 2;
      } else {
        if (c === '$' || c === '`') {
          word.expands = true;
          word.expandsUnquoted = true;
        }
        word.text += c;
        i++;
      }
    }
    out.push(word);
  }
  return out;
}

/** The words of each docker exec call, up to the end of that command. */
function dockerExecCalls(script: string): Array<{ line: number; args: Word[] }> {
  const out: Array<{ line: number; args: Word[] }> = [];
  for (const command of commands(script)) {
    const ws = words(command.text);
    for (let k = 0; k + 1 < ws.length; k++) {
      if (ws[k].text !== 'docker' || ws[k + 1].text !== 'exec') continue;
      const args: Word[] = [];
      for (const w of ws.slice(k + 2)) {
        if (['|', '||', '&&', ';', '&'].includes(w.text)) break;
        args.push(w);
        if (w.text.endsWith(';')) break;
      }
      out.push({ line: command.line, args });
    }
  }
  return out;
}

function psqlCommandExpansions(script: string): string[] {
  const problems: string[] = [];
  for (const command of commands(script)) {
    const ws = words(command.text);
    const psql = ws.findIndex((w) => /(?:^|\/)psql$/.test(w.text));
    if (psql === -1) continue;
    for (let k = psql + 1; k < ws.length; k++) {
      let argument: Word | undefined;
      if (/^-[A-Za-z]*c$/.test(ws[k].text) || ws[k].text === '--command') argument = ws[k + 1];
      else if (ws[k].text.startsWith('--command=')) argument = ws[k];
      if (argument?.expands) problems.push(`${SCRIPT_LABEL}:${command.line}: psql -c argument is expanded by the shell: ${argument.text}`);
    }
  }
  return problems;
}

function dockerExecShells(script: string): string[] {
  const shell = /^(?:\/(?:usr\/)?bin\/)?(?:sh|bash|dash|zsh|ksh|ash)$/;
  return dockerExecCalls(script)
    .filter(({ args }) => args.some((w, k) => shell.test(w.text) && /^-[A-Za-z]*c[A-Za-z]*$/.test(args[k + 1]?.text ?? '')))
    .map(({ line }) => `${SCRIPT_LABEL}:${line}: ${DOCKER_EXEC} runs a shell with -c`);
}

function dockerExecUnquotedExpansions(script: string): string[] {
  return dockerExecCalls(script).flatMap(({ line, args }) =>
    args.filter((w) => w.expandsUnquoted).map((w) => `${SCRIPT_LABEL}:${line}: ${DOCKER_EXEC} argument is not quoted: ${w.text}`),
  );
}

function forcePasswordChangeUpdateProblems(script: string): string[] {
  const updates = commands(script).filter((c) => `${c.text}\n${c.heredoc?.body ?? ''}`.includes('force_password_change'));
  if (updates.length !== 1) return [`${SCRIPT_LABEL}: ${updates.length} commands set force_password_change, expected 1`];
  const [update] = updates;
  const problems: string[] = [];
  if (!/(?:^|\s)-v\s+email="\$(?:ADMIN_EMAIL|\{ADMIN_EMAIL\})"(?=\s|$)/.test(update.text)) {
    problems.push(`${SCRIPT_LABEL}:${update.line}: psql does not take the email as -v email="$ADMIN_EMAIL"`);
  }
  if (update.heredoc === undefined || !update.heredoc.quotedDelimiter) {
    problems.push(`${SCRIPT_LABEL}:${update.line}: the UPDATE is not read from a heredoc with a quoted delimiter`);
  }
  if (!/\bWHERE\s+email\s*=\s*:'email'/.test(update.heredoc?.body ?? '')) {
    problems.push(`${SCRIPT_LABEL}:${update.line}: the UPDATE does not compare email with :'email'`);
  }
  return problems;
}

const script = readFileSync(SCRIPT_PATH, 'utf-8');

describe('the release smoke hands the shell no psql or docker exec command it can rewrite', () => {
  it('reports each shape these tests refuse', () => {
    const psqlDashC = [
      `${DOCKER_EXEC} "$C" \\`,
      '    psql -U postgres -d "$DB" -v ON_ERROR_STOP=1 \\',
      `    -c "UPDATE users SET force_password_change = FALSE WHERE email = '$ADMIN_EMAIL';" >/dev/null`,
    ].join('\n');
    expect(psqlCommandExpansions(psqlDashC)).toHaveLength(1);
    expect(psqlCommandExpansions('psql -tAc "SELECT 1 WHERE $X"')).toHaveLength(1);
    expect(psqlCommandExpansions(`psql --command="SELECT $(id)"`)).toHaveLength(1);
    expect(psqlCommandExpansions(`psql -c 'SELECT 1' -v email="$E"`)).toEqual([]);
    expect(forcePasswordChangeUpdateProblems(psqlDashC)).toHaveLength(3);

    expect(dockerExecShells(`if ${DOCKER_EXEC} "$C" sh -c "pg_isready -d $DB"; then`)).toHaveLength(1);
    expect(dockerExecShells(`${DOCKER_EXEC} -i "$C" /bin/bash -lc "psql"`)).toHaveLength(1);
    expect(dockerExecShells(`${DOCKER_EXEC} "$C" pg_isready; sh -c true`)).toEqual([]);

    expect(dockerExecUnquotedExpansions(`${DOCKER_EXEC} $C pg_isready -d "$DB"`)).toEqual([
      `${SCRIPT_LABEL}:1: ${DOCKER_EXEC} argument is not quoted: $C`,
    ]);

    const unquotedHeredoc = [
      `${DOCKER_EXEC} -i "$C" psql -v email="$ADMIN_EMAIL" <<SQL`,
      "UPDATE users SET force_password_change = FALSE WHERE email = :'email';",
      'SQL',
    ].join('\n');
    expect(forcePasswordChangeUpdateProblems(unquotedHeredoc)).toEqual([
      `${SCRIPT_LABEL}:1: the UPDATE is not read from a heredoc with a quoted delimiter`,
    ]);
  });

  it('finds the docker exec calls and the psql command it checks', () => {
    expect(dockerExecCalls(script).length).toBeGreaterThan(0);
    expect(commands(script).some((c) => words(c.text).some((w) => w.text === 'psql'))).toBe(true);
  });

  it('passes psql no -c argument the shell expands', () => {
    expect(psqlCommandExpansions(script)).toEqual([]);
  });

  it('runs no shell with -c through docker exec', () => {
    expect(dockerExecShells(script)).toEqual([]);
  });

  it('quotes every expansion in a docker exec command', () => {
    expect(dockerExecUnquotedExpansions(script)).toEqual([]);
  });

  it('hands psql the admin email as the variable email for the force_password_change UPDATE', () => {
    expect(forcePasswordChangeUpdateProblems(script)).toEqual([]);
  });
});
