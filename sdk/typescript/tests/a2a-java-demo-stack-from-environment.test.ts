import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import { dirname, join } from 'path';
import { spawnSync } from 'child_process';

/**
 * The A2A multi-agent demo's Java program reaches the stack the compose files
 * name.
 *
 * docker-compose.yml and docker-compose.quickstart.yml take the backend's host
 * port from AIM_BACKEND_PORT (default 8080) and the dashboard's from
 * AIM_FRONTEND_PORT (3000). A2AMultiAgentDemo.java used localhost:8080 when
 * AIM_URL was not set, and printed its dashboard links on the backend URL,
 * which serves no dashboard. Against a stack started on other ports it reached
 * nothing, or, beside a default stack on the same host, that stack.
 *
 * The demo's own sources are compiled once beside stand-ins for the two agents,
 * the AIM SDK class they use and Gson, so every backend URL the demo hands an
 * agent and every URL it prints is recorded. Each case then runs it from the
 * java/ directory of a scratch tree shaped like the repository, the directory
 * the README runs it from, with the repository's .env three levels up.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const DEMO_PACKAGE = join('org', 'opena2a', 'demo');
const DEMO_SRC = join(REPO_ROOT, 'examples', 'a2a-multi-agent-demo', 'java', 'src', 'main', 'java', DEMO_PACKAGE);
const DEMO_FILES = ['A2AMultiAgentDemo.java', 'StackEnv.java'];

/** Writes one line to the call log. */
const STUB_LOG = `package org.opena2a.demo;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;

final class StubLog {
    static void write(String line) {
        try {
            Files.writeString(Path.of(System.getenv("STUB_LOG")), line + "\\n",
                    StandardOpenOption.CREATE, StandardOpenOption.APPEND);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}
`;

const FAKE_AGENT_CARD = `package org.opena2a.aim.a2a;

public class A2AAgentCard {
    public static class Skill {
        private final String name;

        public Skill(String name) {
            this.name = name;
        }

        public String getName() {
            return name;
        }

        public String getDescription() {
            return name + " stand-in";
        }
    }
}
`;

/** Records the backend URL it is given. */
const FAKE_ANALYSIS_AGENT = `package org.opena2a.demo;

import java.util.List;
import org.opena2a.aim.a2a.A2AAgentCard;

public class AnalysisAgent {
    public static final List<A2AAgentCard.Skill> SKILLS = List.of(new A2AAgentCard.Skill("sentiment-analysis"));

    public AnalysisAgent(String aimUrl) {
        StubLog.write("analysis-agent " + aimUrl);
    }

    public void initialize() {}

    public A2AAgentCard registerAgentCard() {
        return new A2AAgentCard();
    }

    public String getAgentId() {
        return "analysis-agent-stub-id";
    }
}
`;

/** Records the backend URL it is given; discovery finds nothing. */
const FAKE_RESEARCH_AGENT = `package org.opena2a.demo;

import java.util.List;
import java.util.Map;
import org.opena2a.aim.a2a.A2AAgentCard;

public class ResearchAgent {
    public static final List<A2AAgentCard.Skill> SKILLS = List.of(new A2AAgentCard.Skill("web-research"));

    public ResearchAgent(String aimUrl) {
        StubLog.write("research-agent " + aimUrl);
    }

    public void initialize() {}

    public A2AAgentCard registerAgentCard() {
        return new A2AAgentCard();
    }

    public String getAgentId() {
        return "research-agent-stub-id";
    }

    public Map<String, Object> discoverAnalysisAgent(String intent, double minTrust) {
        return null;
    }

    public void getTrustScore() {}

    public void getPeerTrust(String peerAgentId) {}

    public void checkSecurityPolicies(String targetAgentId, String skillId) {}

    public boolean requestUserConsent(String userId, String recipientAgentId, List<String> dataTypes, String purpose) {
        return true;
    }

    public Map<String, Object> signAndCallSkill(String targetAgentId, String skillId, Map<String, Object> data,
            AnalysisAgent analysisAgent) {
        return Map.of("sentiment", "positive", "trend", "up");
    }

    public Map<String, Object> attestSkillQuality(String targetAgentId, String skillId, double confidence,
            Map<String, Object> evidence) {
        return Map.of();
    }
}
`;

const FAKE_GSON = `package com.google.gson;

import java.lang.reflect.Type;

public class Gson {
    public <T> T fromJson(String json, Type typeOfT) {
        throw new UnsupportedOperationException("Gson stand-in");
    }
}
`;

const FAKE_TYPE_TOKEN = `package com.google.gson.reflect;

import java.lang.reflect.Type;

public class TypeToken<T> {
    protected TypeToken() {}

    public Type getType() {
        return Object.class;
    }
}
`;

const STUBS: Record<string, string> = {
  [join(DEMO_PACKAGE, 'StubLog.java')]: STUB_LOG,
  [join(DEMO_PACKAGE, 'AnalysisAgent.java')]: FAKE_ANALYSIS_AGENT,
  [join(DEMO_PACKAGE, 'ResearchAgent.java')]: FAKE_RESEARCH_AGENT,
  [join('org', 'opena2a', 'aim', 'a2a', 'A2AAgentCard.java')]: FAKE_AGENT_CARD,
  [join('com', 'google', 'gson', 'Gson.java')]: FAKE_GSON,
  [join('com', 'google', 'gson', 'reflect', 'TypeToken.java')]: FAKE_TYPE_TOKEN,
};

interface Stack {
  name: string;
  env?: Record<string, string>;
  dotenv?: string;
  backend: string;
  frontend: string;
}

const DOTENV = 'AIM_PROJECT=dotlab\nAIM_BACKEND_PORT=18282\nAIM_FRONTEND_PORT=13232\n';

const STACKS: Stack[] = [
  { name: 'nothing set', backend: '8080', frontend: '3000' },
  {
    name: 'the environment',
    env: { AIM_BACKEND_PORT: '18181', AIM_FRONTEND_PORT: '13131' },
    backend: '18181',
    frontend: '13131',
  },
  { name: "the repository's .env", dotenv: DOTENV, backend: '18282', frontend: '13232' },
  {
    name: 'the environment over .env',
    dotenv: DOTENV,
    env: { AIM_FRONTEND_PORT: '13333' },
    backend: '18282',
    frontend: '13333',
  },
];

const DASHBOARD_TABS = ['', '?tab=cards', '?tab=consent', '?tab=tasks', '?tab=trust', '?tab=skills'];

interface Run {
  status: number | null;
  output: string;
  calls: string[];
}

let scratch = '';
let classes = '';
let caseCount = 0;

beforeAll(() => {
  scratch = mkdtempSync(join(tmpdir(), 'a2a-java-demo-'));
  const src = join(scratch, 'src');
  classes = join(scratch, 'classes');
  const sources: string[] = [];
  // A file the demo does not have yet is left out.
  for (const f of DEMO_FILES) {
    if (!existsSync(join(DEMO_SRC, f))) continue;
    const to = join(src, DEMO_PACKAGE, f);
    mkdirSync(dirname(to), { recursive: true });
    writeFileSync(to, readFileSync(join(DEMO_SRC, f)));
    sources.push(to);
  }
  for (const [path, body] of Object.entries(STUBS)) {
    const to = join(src, path);
    mkdirSync(dirname(to), { recursive: true });
    writeFileSync(to, body);
    sources.push(to);
  }
  // The demo's pom.xml compiles for Java 17.
  const javac = spawnSync('javac', ['--release', '17', '-d', classes, ...sources], { encoding: 'utf-8' });
  if (javac.error) throw javac.error;
  if (javac.status !== 0) throw new Error(`javac failed:\n${javac.stdout}${javac.stderr}`);
}, 120_000);

afterAll(() => {
  if (scratch) rmSync(scratch, { recursive: true, force: true });
});

/** Runs the demo from the java/ directory of a fresh scratch tree. */
function run(opts: { stack?: Stack; env?: Record<string, string> } = {}): Run {
  const root = join(scratch, `case-${++caseCount}`);
  const demo = join(root, 'examples', 'a2a-multi-agent-demo', 'java');
  mkdirSync(demo, { recursive: true });
  if (opts.stack?.dotenv !== undefined) writeFileSync(join(root, '.env'), opts.stack.dotenv);
  const log = join(root, 'calls.log');
  writeFileSync(log, '');

  const result = spawnSync('java', ['-cp', classes, 'org.opena2a.demo.A2AMultiAgentDemo'], {
    cwd: demo,
    encoding: 'utf-8',
    env: {
      PATH: process.env.PATH ?? '',
      ...(process.env.JAVA_HOME ? { JAVA_HOME: process.env.JAVA_HOME } : {}),
      HOME: join(root, 'home'),
      LANG: 'en_US.UTF-8',
      STUB_LOG: log,
      ...(opts.stack?.env ?? {}),
      ...(opts.env ?? {}),
    },
  });
  if (result.error) throw result.error;
  return {
    status: result.status,
    output: `${result.stdout}${result.stderr}`,
    calls: readFileSync(log, 'utf-8').split('\n').filter((l) => l !== ''),
  };
}

/** Every localhost port the output names. */
function localhostPorts(output: string): string[] {
  return [...output.matchAll(/localhost:(\d+)/g)].map((m) => m[1]);
}

function expectDashboardLinks(output: string, frontend: string): void {
  for (const tab of DASHBOARD_TABS) expect(output).toContain(`${frontend}/dashboard/a2a${tab}\n`);
}

describe.each(STACKS)('the Java A2A demo with $name', (stack) => {
  it('hands both agents the backend port and links the dashboard port', () => {
    const backend = `http://localhost:${stack.backend}`;
    const r = run({ stack });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual([`analysis-agent ${backend}`, `research-agent ${backend}`]);
    expectDashboardLinks(r.output, `http://localhost:${stack.frontend}`);
    for (const port of localhostPorts(r.output)) expect(port).toBe(stack.frontend);
  });
});

describe('the Java A2A demo with a backend URL given', () => {
  it('AIM_URL wins over AIM_BACKEND_PORT for the backend; the links use the dashboard port', () => {
    const r = run({
      env: { AIM_BACKEND_PORT: '18181', AIM_FRONTEND_PORT: '13131', AIM_URL: 'http://localhost:18999' },
    });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual(['analysis-agent http://localhost:18999', 'research-agent http://localhost:18999']);
    expectDashboardLinks(r.output, 'http://localhost:13131');
    for (const port of localhostPorts(r.output)) expect(port).toBe('13131');
  });

  it('an empty AIM_URL counts as unset', () => {
    const r = run({ env: { AIM_BACKEND_PORT: '18181', AIM_URL: '' } });
    expect(r.status, r.output).toBe(0);
    expect(r.calls).toEqual(['analysis-agent http://localhost:18181', 'research-agent http://localhost:18181']);
  });
});

describe("the Java A2A demo's sources", () => {
  it('name no fixed backend or dashboard port outside comments', () => {
    const lines = readFileSync(join(DEMO_SRC, 'A2AMultiAgentDemo.java'), 'utf-8').split('\n');
    const fixed = lines.filter((l) => /"[^"]*localhost:(8080|3000)\b/.test(l));
    expect(fixed).toEqual([]);
  });
});
