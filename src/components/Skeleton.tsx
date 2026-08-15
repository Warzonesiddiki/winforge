export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-lg bg-white/10 ${className}`} />;
}

export function CardSkeleton() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <Skeleton className="mb-3 h-5 w-32" />
      <Skeleton className="mb-2 h-4 w-full" />
      <Skeleton className="h-4 w-3/4" />
      <div className="mt-4 flex gap-2">
        <Skeleton className="h-6 w-16" />
        <Skeleton className="h-6 w-12" />
      </div>
    </div>
  );
}

export function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="overflow-hidden rounded-2xl border border-white/10">
      <div className="bg-white/5 px-4 py-3">
        <Skeleton className="h-4 w-full" />
      </div>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex gap-4 border-t border-white/5 px-4 py-3">
          <Skeleton className="h-4 w-1/4" />
          <Skeleton className="h-4 w-1/3" />
          <Skeleton className="h-4 w-1/6" />
          <Skeleton className="ml-auto h-4 w-16" />
        </div>
      ))}
    </div>
  );
}

export function GaugeSkeleton() {
  return (
    <div className="flex flex-col items-center gap-3">
      <Skeleton className="h-40 w-40 rounded-full" />
      <Skeleton className="h-4 w-24" />
    </div>
  );
}

export function StatCardSkeleton() {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
      <Skeleton className="mb-2 h-3 w-20" />
      <Skeleton className="h-7 w-16" />
    </div>
  );
}
