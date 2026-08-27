import { redirect } from "next/navigation";

/**
 * Retired orphan (IA consolidation Stage 1): this page was a strict subset of
 * the full policy surface — enforcement settings only, no policy CRUD. One
 * Policies surface now lives under Security. Admin-gated at the edge; the
 * subset page had no navigation path, so deep links are the only traffic.
 */
export default function SecurityPoliciesRedirect() {
  redirect("/dashboard/admin/security-policies");
}
