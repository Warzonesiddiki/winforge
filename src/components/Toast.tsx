"use client";

import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

export type ToastType = "success" | "error" | "warning" | "info";

interface Toast {
  id: string;
  type: ToastType;
  title: string;
  message?: string;
}

interface ToastContextValue {
  toasts: Toast[];
  addToast: (type: ToastType, title: string, message?: string) => void;
  removeToast: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((type: ToastType, title: string, message?: string) => {
    const id = Math.random().toString(36).slice(2);
    setToasts((prev) => [...prev, { id, type, title, message }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 5000);
  }, []);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toasts, addToast, removeToast }}>
      {children}
      <ToastContainer toasts={toasts} onRemove={removeToast} />
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

function ToastContainer({ toasts, onRemove }: { toasts: Toast[]; onRemove: (id: string) => void }) {
  if (toasts.length === 0) return null;

  const typeStyles: Record<ToastType, string> = {
    success: "border-emerald-500/50 bg-emerald-500/10 text-emerald-300",
    error: "border-red-500/50 bg-red-500/10 text-red-300",
    warning: "border-amber-500/50 bg-amber-500/10 text-amber-300",
    info: "border-sky-500/50 bg-sky-500/10 text-sky-300",
  };

  const icons: Record<ToastType, string> = {
    success: "✓",
    error: "✕",
    warning: "⚠",
    info: "ℹ",
  };

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={`flex items-start gap-3 rounded-xl border px-4 py-3 shadow-lg backdrop-blur-sm animate-in slide-in-from-right ${typeStyles[toast.type]}`}
          style={{ minWidth: 280, maxWidth: 400 }}
        >
          <span className="mt-0.5 text-lg">{icons[toast.type]}</span>
          <div className="flex-1">
            <p className="font-medium">{toast.title}</p>
            {toast.message && <p className="mt-0.5 text-sm opacity-80">{toast.message}</p>}
          </div>
          <button onClick={() => onRemove(toast.id)} className="text-lg opacity-60 hover:opacity-100">
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
