import { cn } from "@/lib/utils";

function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("animate-pulse rounded-inset-sm bg-track", className)}
      {...props}
    />
  );
}

export { Skeleton };
