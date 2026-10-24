# ui-feedback

Telling people what happened, in Bingkai Navy's design. Builds on `ui-core` and `ui-forms`.
Registers a `provider` (`FeedbackProvider`) that wraps the app once: an error boundary showing a
crash card (with "versi baru" detection after a deploy), the global loading bar for running
http-client requests, and the toast container.

| Piece | Use |
|---|---|
| `notify.success / info / warning / error(err)` | Toasts; `error` shows the API's message |
| `ConfirmDialog` | Before something hard to undo; async-aware; shows the error and stays open if it fails |
| `ReasonDialog` | A confirmation that requires a reason (cancel, void) |
| `GlobalLoader` | Thin bar at the top while requests run (mounted by the provider) |
| `ErrorBoundary` · `CrashCard` | Render failures as a card with an error ID, not a blank page |

A toast reports something that happened; a `Notice` (ui-core) is something the user must read.
