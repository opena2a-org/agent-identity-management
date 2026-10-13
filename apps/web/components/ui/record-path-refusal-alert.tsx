import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import type { RecordPathRefusal } from "@/lib/record-path-refusal";

interface RecordPathRefusalAlertProps {
  refusal: RecordPathRefusal | null;
  className?: string;
}

/**
 * A change the server refused because its audit record could not be written, shown inside
 * the dialog or form where the action was taken. The default variant, not the destructive
 * one: the person did nothing wrong. role="alert" comes from Alert.
 */
export function RecordPathRefusalAlert({ refusal, className = "" }: RecordPathRefusalAlertProps) {
  if (!refusal) return null;
  return (
    <Alert className={className}>
      <AlertCircle className="h-4 w-4" aria-hidden="true" />
      <AlertTitle>{refusal.title}</AlertTitle>
      <AlertDescription>{refusal.description}</AlertDescription>
    </Alert>
  );
}
