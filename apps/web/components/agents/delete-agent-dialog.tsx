"use client";

import { useEffect, useState } from "react";
import type { MouseEvent } from "react";
import { Loader2 } from "lucide-react";
import { api } from "@/lib/api";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { ActionOutcome } from "@/components/ui/action-outcome";
import type { ActionFailure } from "@/lib/action-outcome";
import {
  AGENT_DELETE_IRREVERSIBLE_TEXT,
  AGENT_DELETE_KEPT_TEXT,
  agentDeleteFailure,
  agentDeleteRemovesText,
} from "@/lib/agent-delete";

export interface DeleteAgentTarget {
  id: string;
  name: string;
}

interface DeleteAgentDialogProps {
  agent: DeleteAgentTarget | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  // Called once the route has answered success. The caller closes the dialog.
  onDeleted: (agent: DeleteAgentTarget) => void;
}

// The one delete dialog for an agent, used by the agent list and the agent page. A failed
// delete keeps the dialog open and states the reason and next step inside it, in an alert,
// so focus stays in the dialog and assistive technology reads the reason.
export function DeleteAgentDialog({
  agent,
  open,
  onOpenChange,
  onDeleted,
}: DeleteAgentDialogProps) {
  const [deleting, setDeleting] = useState(false);
  const [failure, setFailure] = useState<ActionFailure | null>(null);

  useEffect(() => {
    if (open) setFailure(null);
  }, [open, agent?.id]);

  const handleOpenChange = (next: boolean) => {
    // A delete in flight cannot be cancelled, so the dialog stays until it answers.
    if (deleting && !next) return;
    onOpenChange(next);
  };

  const handleDelete = async (e: MouseEvent<HTMLButtonElement>) => {
    // The action closes the dialog by default; it closes only after a success.
    e.preventDefault();
    if (!agent || deleting) return;
    setDeleting(true);
    setFailure(null);
    try {
      await api.deleteAgent(agent.id);
      setDeleting(false);
      onDeleted(agent);
    } catch (err) {
      console.error("Failed to delete agent:", err);
      setFailure(agentDeleteFailure(err));
      setDeleting(false);
    }
  };

  return (
    <AlertDialog open={open && agent !== null} onOpenChange={handleOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete agent</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-2">
              <p>{agent ? agentDeleteRemovesText(agent.name) : ""}</p>
              <p>{AGENT_DELETE_KEPT_TEXT}</p>
              <p>{AGENT_DELETE_IRREVERSIBLE_TEXT}</p>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <ActionOutcome outcome={failure} />
        <AlertDialogFooter>
          <AlertDialogCancel
            aria-disabled={deleting || undefined}
            onClick={(e) => {
              if (deleting) e.preventDefault();
            }}
          >
            Cancel
          </AlertDialogCancel>
          {/* aria-disabled rather than disabled: a disabled button drops focus out of
              the dialog while the request runs. */}
          <AlertDialogAction
            onClick={handleDelete}
            aria-disabled={deleting || undefined}
            className="rounded-pill bg-danger hover:brightness-95"
          >
            {deleting && <Loader2 className="mr-1 h-4 w-4 animate-spin" aria-hidden="true" />}
            {deleting ? "Deleting..." : "Delete"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
