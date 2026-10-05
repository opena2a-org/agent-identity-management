import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

// The element every chart renders inside: one image to assistive technology,
// labelled with the values it draws. It fills its parent unless sized here.
export function ChartFrame({
  label,
  className,
  children,
}: {
  label: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div role="img" aria-label={label} className={cn("h-full w-full", className)}>
      {children}
    </div>
  );
}
