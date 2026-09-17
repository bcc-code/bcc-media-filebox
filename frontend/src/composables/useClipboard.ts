/**
 * Copies text, reporting whether it actually worked.
 *
 * Deliberately not @zag-js/clipboard: that machine transitions to "copied"
 * optimistically and never surfaces a rejection, so it would keep the bug this
 * fixes. The one thing it does add — a fallback for when
 * navigator.clipboard is missing — is the ten lines below.
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  // navigator.clipboard requires a secure context, so it is undefined on any
  // plain-http origin. The old code called it optionally and swallowed the
  // rejection, which reported success while copying nothing.
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // Permission denied, or the document lost focus mid-write. Fall through.
    }
  }
  return legacyCopy(text)
}

/** execCommand path for non-secure origins and older browsers. */
function legacyCopy(text: string): boolean {
  if (!document.body) return false
  const node = document.createElement('textarea')
  node.value = text
  // Off-screen but still selectable; `hidden` or display:none would not be.
  node.setAttribute('readonly', '')
  node.style.cssText =
    'position:fixed;top:-9999px;opacity:0;pointer-events:none'
  document.body.appendChild(node)
  try {
    node.select()
    node.setSelectionRange(0, text.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    document.body.removeChild(node)
  }
}
