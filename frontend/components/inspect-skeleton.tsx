"use client";

import { Skeleton } from "@/components/ui/skeleton";

export function InspectSkeleton() {
  return (
    <div
      aria-busy="true"
      aria-label="Loading YouTube metadata"
      className="border border-border bg-card"
    >
      <div className="grid grid-cols-1 lg:grid-cols-12">
        <div className="border-b border-border p-5 lg:col-span-5 lg:border-r lg:border-b-0">
          <Skeleton className="aspect-video w-full" />
          <Skeleton className="mt-4 h-7 w-4/5" />
          <Skeleton className="mt-2 h-4 w-2/5" />
        </div>
        <div className="space-y-3 p-5 lg:col-span-7">
          <div className="flex items-center justify-between">
            <Skeleton className="h-8 w-40" />
            <Skeleton className="h-6 w-24" />
          </div>
          <div className="space-y-2 pt-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
          <div className="pt-2">
            <Skeleton className="h-11 w-full" />
          </div>
        </div>
      </div>
    </div>
  );
}
