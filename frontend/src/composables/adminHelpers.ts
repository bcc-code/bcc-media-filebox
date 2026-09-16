// Small presentation helpers used across the admin tabs/modals. Kept
// framework-agnostic — pure functions, no reactive state.

export function formatBytes(n: number): string {
  if (!isFinite(n) || n <= 0) return '—'
  const gb = n / (1024 ** 3)
  if (gb >= 1024) return (gb / 1024).toFixed(2) + ' TB'
  if (gb >= 1) return gb.toFixed(1) + ' GB'
  const mb = n / (1024 ** 2)
  if (mb >= 1) return mb.toFixed(1) + ' MB'
  const kb = n / 1024
  if (kb >= 1) return kb.toFixed(1) + ' KB'
  return n + ' B'
}

export function relTime(iso: string): string {
  if (!iso) return '—'
  const now = Date.now()
  const t = new Date(iso).getTime()
  if (isNaN(t)) return '—'
  const days = Math.floor((now - t) / 86400000)
  if (days <= 0) return 'today'
  if (days === 1) return 'yesterday'
  if (days < 7) return days + ' days ago'
  if (days < 30) return Math.round(days / 7) + ' weeks ago'
  if (days < 365) return Math.round(days / 30) + ' months ago'
  return Math.round(days / 365) + ' years ago'
}

export function initials(name: string): string {
  if (!name) return '?'
  return name
    .split(' ')
    .filter(p => !/^[a-z]+:$/i.test(p))
    .slice(0, 2)
    .map(p => p[0])
    .join('')
    .toUpperCase()
}

export function providerLabel(p: string): string {
  switch (p) {
    case 'azure':
    case 'microsoft':
      return 'Azure AD'
    case 'bcc':
      return 'BCC Login'
    case 'guest':
      return 'Guest'
    default:
      return p || '—'
  }
}

export function providerColor(p: string): string {
  switch (p) {
    case 'azure':
    case 'microsoft':
      return '#5da5ff'
    case 'bcc':
      return '#9bbcff'
    case 'guest':
      return 'oklch(0.80 0.14 75)'
    default:
      return 'var(--color-ink-2)'
  }
}

const palette = [
  ['oklch(0.72 0.10 250)', 'oklch(0.62 0.12 280)'],
  ['oklch(0.74 0.12 160)', 'oklch(0.65 0.11 200)'],
  ['oklch(0.78 0.13 80)', 'oklch(0.70 0.14 30)'],
  ['oklch(0.75 0.10 320)', 'oklch(0.65 0.12 290)'],
  ['oklch(0.78 0.10 140)', 'oklch(0.70 0.11 180)'],
  ['oklch(0.70 0.10 50)', 'oklch(0.62 0.12 20)'],
  ['oklch(0.74 0.12 10)', 'oklch(0.66 0.13 340)'],
  ['oklch(0.76 0.11 110)', 'oklch(0.68 0.12 70)'],
  ['oklch(0.72 0.10 220)', 'oklch(0.64 0.12 260)'],
  ['oklch(0.72 0.08 200)', 'oklch(0.62 0.10 240)'],
]

export function avatarBg(seed: string): string {
  let hash = 0
  for (let i = 0; i < seed.length; i++) hash = (hash * 31 + seed.charCodeAt(i)) | 0
  const [a, b] = palette[Math.abs(hash) % palette.length]
  return `linear-gradient(135deg, ${a}, ${b})`
}
