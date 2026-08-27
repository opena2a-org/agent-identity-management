import { redirect } from "next/navigation";

/**
 * Retired orphan (IA consolidation Stage 1): this standalone form drifted from
 * the register modal's required fields. Registration has one surface — the
 * modal on the agents list, which ?register=1 opens.
 */
export default function NewAgentRedirect() {
  redirect("/dashboard/agents?register=1");
}
