import * as toast from '@zag-js/toast'

// One store for the whole app, mirroring how useProviders/useAdmin hold
// module-level state. <UiToaster /> renders whatever lands here, so anything —
// a composable, a component, a plain module — can notify without prop drilling.
export const toastStore = toast.createStore({
  placement: 'bottom',
  // Listed rather than piled: these are short confirmations, and overlap
  // mode hides all but the front toast until you hover it.
  overlap: false,
  duration: 2400,
  max: 4,
})

/** Transient confirmation, e.g. "Download link copied". */
export function notify(title: string) {
  toastStore.success({ title })
}

/** Something failed. Stays up longer — the user may need to read it. */
export function notifyError(title: string) {
  toastStore.error({ title, duration: 5000 })
}

export function useToast() {
  return { notify, notifyError, store: toastStore }
}
