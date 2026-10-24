import { Component } from "react";
import type { ErrorInfo, ReactNode } from "react";
import { CrashCard } from "@/shared/components/ui/crash-card";

// A lazy page whose chunk vanished after a deploy fails with one of these messages.
const STALE = /Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module/i;

interface State {
  error: Error | null;
  errorId: string;
}

/** Catches a render error below it and shows CrashCard with an ID to report. */
export class ErrorBoundary extends Component<{ children: ReactNode; fullScreen?: boolean }, State> {
  state: State = { error: null, errorId: "" };

  static getDerivedStateFromError(error: Error): State {
    return { error, errorId: Math.random().toString(36).slice(2, 10).toUpperCase() };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`[${this.state.errorId}]`, error, info.componentStack);
  }

  render() {
    if (!this.state.error) return this.props.children;
    return <CrashCard errorId={this.state.errorId} stale={STALE.test(this.state.error.message)} fullScreen={this.props.fullScreen} />;
  }
}
