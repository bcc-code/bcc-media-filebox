import { ref } from 'vue'

export interface ConfirmRequest {
  /** The question. Becomes the dialog's accessible name. */
  title: string
  /** What will actually happen — the consequence the user is agreeing to. */
  body?: string
  /** Label for the confirming action. Name the action, not "OK". */
  confirmLabel?: string
  cancelLabel?: string
  /** Styles the confirm button as destructive. */
  danger?: boolean
}

interface PendingConfirm extends ConfirmRequest {
  resolve: (confirmed: boolean) => void
}

// One queue slot, like the native confirm() this replaces: a second request
// while one is open would leave the first promise dangling.
export const pending = ref<PendingConfirm | null>(null)

/**
 * Promise-based replacement for window.confirm(). Named distinctly rather than
 * shadowing the global, so a forgotten import is a compile error instead of a
 * silent fall back to the OS dialog.
 */
export function confirmAction(request: ConfirmRequest): Promise<boolean> {
  // Resolve any in-flight request as cancelled rather than dropping it.
  pending.value?.resolve(false)
  return new Promise<boolean>((resolve) => {
    pending.value = { danger: true, ...request, resolve }
  })
}

export function settle(confirmed: boolean) {
  const current = pending.value
  pending.value = null
  current?.resolve(confirmed)
}
