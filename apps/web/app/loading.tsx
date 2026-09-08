import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
  return (
    <div className="flex min-h-screen items-center justify-center p-6" aria-busy="true" aria-label="Loading">
      <div className="glass w-full max-w-md space-y-3 p-6">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-7 w-2/3" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
      </div>
    </div>
  );
}
