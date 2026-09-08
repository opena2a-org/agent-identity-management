/**
 * The SDK quickstart content shared by the dashboard (zero state and side panel)
 * and cloud's get-started page. The SDK page, the register-agent modal, the agent
 * success page and sdk-setup-guide still carry their own install copy; new
 * install-teaching surfaces should read this table instead of adding a fifth.
 *
 * `origin` is this deployment's own web origin, which proxies /api/* to the
 * backend; pass "" for the hosted default (the SDK's built-in URL) or during
 * prerender, and each command falls back accordingly.
 */

export type SdkLang = "python" | "typescript" | "java";

export interface SdkTab {
  key: SdkLang;
  label: string;
  /** Terminal line that installs the SDK and, where the SDK has one, signs in. */
  install: (origin: string) => string;
  /** Minimal working example against the shipped SDK. */
  code: (origin: string) => string;
  docsHref: string;
  docsLabel: string;
  /** Shown under the example when the flow needs explaining. */
  note?: string;
}

export const SDK_TABS: readonly SdkTab[] = [
  {
    key: "python",
    label: "Python",
    install: (origin) => (origin ? `pip install aim-sdk && aim-sdk login --url ${origin}` : "pip install aim-sdk && aim-sdk login"),
    docsHref: "https://opena2a.org/docs/tutorials/sdk-quickstart",
    docsLabel: "SDK quickstart",
    note:
      "The SDK creates an Ed25519 keypair on your machine, registers the agent under your account and stores the credentials in ~/.aim/. The private key never leaves your machine.",
    code: () => `from aim_sdk import secure

agent = secure("my-first-agent")

@agent.perform_action(capability="db:read")
def get_customer(customer_id):
    return db.query(
        "SELECT * FROM customers WHERE id = ?",
        customer_id,
    )`,
  },
  {
    // TypeScript shows the AIMClient API: @opena2a/aim-sdk 1.3.0 exports no
    // secure() and its only bin is aim-arp, not a login command, so a pip-style
    // command quickstart would not run there.
    key: "typescript",
    label: "TypeScript",
    install: () => "npm install @opena2a/aim-sdk",
    docsHref: "https://opena2a.org/docs/tutorials/sdk-quickstart",
    docsLabel: "SDK quickstart",
    code: (origin) => `import { AIMClient, AgentType } from "@opena2a/aim-sdk";

const client = new AIMClient({
  baseUrl: "${origin || "https://api.aim.opena2a.org"}",
  apiKey: process.env.AIM_API_KEY,
});

const agent = await client.registerAgent({
  name: "my-first-agent",
  agentType: AgentType.LANGCHAIN,
  capabilities: ["db:read"],
});

const result = await client.verifyAction({
  action: "db:read",
  resource: "users_table",
});`,
  },
  {
    key: "java",
    label: "Java",
    install: () =>
      "git clone https://github.com/opena2a-org/agent-identity-management.git && mvn -f agent-identity-management/sdk/java -DskipTests install",
    docsHref: "https://github.com/opena2a-org/agent-identity-management/tree/main/sdk/java",
    docsLabel: "Java SDK reference",
    code: () => `import org.opena2a.aim.client.AIMClient;

AIMClient agent = AIMClient.secure("my-first-agent");

User user = agent.performAction("db:read", "users_table", () ->
    userRepository.findById(userId)
);`,
  },
];
