"use client";

import { useEffect, useState } from "react";

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  totalItems?: number;
  itemsPerPage?: number;
}

export function Pagination({ currentPage, totalPages, onPageChange, totalItems, itemsPerPage }: PaginationProps) {
  if (totalPages <= 1) return null;

  const pages: (number | "...")[] = [];
  
  if (totalPages <= 7) {
    for (let i = 1; i <= totalPages; i++) pages.push(i);
  } else {
    pages.push(1);
    if (currentPage > 3) pages.push("...");
    for (let i = Math.max(2, currentPage - 1); i <= Math.min(totalPages - 1, currentPage + 1); i++) {
      pages.push(i);
    }
    if (currentPage < totalPages - 2) pages.push("...");
    pages.push(totalPages);
  }

  return (
    <div className="flex items-center justify-between gap-4">
      {totalItems !== undefined && itemsPerPage !== undefined && (
        <p className="text-sm text-slate-500">
          Showing {Math.min((currentPage - 1) * itemsPerPage + 1, totalItems)}–
          {Math.min(currentPage * itemsPerPage, totalItems)} of {totalItems}
        </p>
      )}
      <div className="flex items-center gap-1">
        <button
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage === 1}
          className="rounded-lg border border-white/10 px-2.5 py-1.5 text-sm text-slate-300 hover:bg-white/5 disabled:opacity-30"
        >
          ←
        </button>
        {pages.map((page, i) =>
          page === "..." ? (
            <span key={`ellipsis-${i}`} className="px-2 text-slate-500">
              …
            </span>
          ) : (
            <button
              key={page}
              onClick={() => onPageChange(page)}
              className={`min-w-[2rem] rounded-lg border px-2.5 py-1.5 text-sm ${
                page === currentPage
                  ? "border-sky-500/50 bg-sky-500/15 text-sky-300"
                  : "border-white/10 text-slate-300 hover:bg-white/5"
              }`}
            >
              {page}
            </button>
          )
        )}
        <button
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage === totalPages}
          className="rounded-lg border border-white/10 px-2.5 py-1.5 text-sm text-slate-300 hover:bg-white/5 disabled:opacity-30"
        >
          →
        </button>
      </div>
    </div>
  );
}

export function usePagination<T>(items: T[], perPage: number = 20) {
  const [page, setPage] = useState(1);
  const totalPages = Math.ceil(items.length / perPage);
  const paginatedItems = items.slice((page - 1) * perPage, page * perPage);
  
  // Reset to page 1 if items change
  const itemsLength = items.length;
  useEffect(() => {
    if (page > Math.ceil(itemsLength / perPage)) {
      setPage(1);
    }
  }, [itemsLength, perPage, page]);

  return {
    items: paginatedItems,
    page,
    setPage,
    totalPages,
    totalItems: items.length,
    perPage,
  };
}
