"use client";

import { Component, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="rounded-2xl border border-red-500/30 bg-red-500/10 p-6">
            <h3 className="text-lg font-semibold text-red-400">Something went wrong</h3>
            <p className="mt-2 text-sm text-red-300/80">{this.state.error?.message ?? "An unexpected error occurred"}</p>
            <button
              onClick={() => this.setState({ hasError: false })}
              className="mt-4 rounded-lg border border-red-500/30 px-3 py-1.5 text-sm text-red-300 hover:bg-red-500/10"
            >
              Try again
            </button>
          </div>
        )
      );
    }

    return this.props.children;
  }
}

export function ErrorState({ title, message, onRetry }: { title: string; message: string; onRetry?: () => void }) {
  return (
    <div className="rounded-2xl border border-red-500/30 bg-red-500/10 p-6 text-center">
      <div className="mb-3 text-4xl">⚠️</div>
      <h3 className="text-lg font-semibold text-red-400">{title}</h3>
      <p className="mt-2 text-sm text-red-300/80">{message}</p>
      {onRetry && (
        <button onClick={onRetry} className="mt-4 rounded-lg border border-red-500/30 px-4 py-2 text-sm text-red-300 hover:bg-red-500/10">
          Retry
        </button>
      )}
    </div>
  );
}

export function EmptyState({ title, message, action }: { title: string; message: string; action?: ReactNode }) {
  return (
    <div className="rounded-2xl border border-white/10 bg-white/[0.02] p-8 text-center">
      <div className="mb-3 text-4xl opacity-40">📭</div>
      <h3 className="text-lg font-medium text-slate-300">{title}</h3>
      <p className="mt-2 text-sm text-slate-500">{message}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
