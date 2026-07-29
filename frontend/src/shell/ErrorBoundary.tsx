// ErrorBoundary is the app's last line of defense against a blank window.
// Before this existed, main.tsx had zero error boundaries anywhere: an
// uncaught render-time exception in ANY component (a bound-service response
// shaped differently than the frontend assumed, a null/undefined property
// access, anything) unmounted the entire React tree with no on-screen
// diagnostic at all -- confirmed live by a real bug (DiagnosticReportView's
// fileLevelIssues being briefly omit-on-empty over the wire, see
// svc_show.go's own doc comment) that turned into exactly this: the desktop
// window going fully blank with nothing to tell an operator what happened.
//
// This does not fix bugs -- it only prevents "silently blank" from being a
// possible failure mode. React error boundaries can only catch render/
// lifecycle exceptions in their child tree (not event handlers, not async
// code, not errors in the boundary itself), so this is deliberately mounted
// as high as possible (wrapping AppShell in App.tsx) to cover the largest
// practical surface.
import { Component, type ErrorInfo, type ReactNode } from "react";
import { RefreshCw } from "lucide-react";

import styles from "./ErrorBoundary.module.css";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

export default class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // eslint-disable-next-line no-console -- the one place this app
    // deliberately logs to the console: there is no other surface left once
    // the render tree has already crashed.
    console.error("GOLC_FRONTEND_UNCAUGHT_RENDER_ERROR", error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) {
      return this.props.children;
    }

    return (
      <div className={styles.screen} role="alert">
        <h1 className={styles.title}>GOLC hit an unexpected error</h1>
        <p className={styles.body}>
          Playback, Art-Net output, and safety controls run independently of this window and are not
          affected. Reloading restarts only this display.
        </p>
        <pre className={styles.detail}>{error.stack ?? error.message}</pre>
        <button
          type="button"
          className={styles.reload}
          onClick={() => window.location.reload()}
        >
          <RefreshCw size={14} aria-hidden="true" />
          Reload
        </button>
      </div>
    );
  }
}
