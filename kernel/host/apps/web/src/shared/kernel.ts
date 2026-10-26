/**
 * The frontend half of the kernel contract — the same number as kernel.Version in
 * apps/api/internal/kernel (see the changelog there). Library modules state the minimum they
 * need; the module installer reads it from here when a project has no Go API.
 */
export const KERNEL_VERSION = 5;
