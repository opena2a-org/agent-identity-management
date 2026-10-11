import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

// The element every chart renders inside: one image to assistive technology,
// labelled with the values it draws. It fills its parent unless sized here.
// `center` is drawn over the middle of the plot (a ring's value); it is part of
// the image, so whatever it says must also be in the label.
export function ChartFrame({
  label,
  className,
  center,
  children,
}: {
  label: string;
  className?: string;
  center?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div role="img" aria-label={label} className={cn("relative h-full w-full", className)}>
      {children}
      {center != null && (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          {center}
        </div>
      )}
    </div>
  );
}
