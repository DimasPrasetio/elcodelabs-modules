/**
 * The one meaning of status color across an app (Bingkai Navy):
 * neutral = not started · info = in progress · warning = needs attention ·
 * success = done · muted = stopped (cancelled, archived) · danger = a problem.
 * A module maps its own statuses onto these; components only know tones.
 */
export type Tone = "success" | "warning" | "danger" | "info" | "neutral" | "muted";
