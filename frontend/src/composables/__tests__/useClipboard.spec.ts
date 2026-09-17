import { describe, it, expect, afterEach, vi } from 'vitest'
import { copyToClipboard } from '../useClipboard'

const original = Object.getOwnPropertyDescriptor(navigator, 'clipboard')

function setClipboard(value: unknown) {
  Object.defineProperty(navigator, 'clipboard', {
    value,
    configurable: true,
    writable: true,
  })
}

afterEach(() => {
  if (original) Object.defineProperty(navigator, 'clipboard', original)
  else setClipboard(undefined)
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('copyToClipboard', () => {
  it('uses the async clipboard when available', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    setClipboard({ writeText })

    await expect(copyToClipboard('https://example.test/s/abc')).resolves.toBe(
      true,
    )
    expect(writeText).toHaveBeenCalledWith('https://example.test/s/abc')
  })

  it('reports failure instead of claiming success', async () => {
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error('denied')) })
    document.execCommand = vi.fn().mockReturnValue(false)

    await expect(copyToClipboard('x')).resolves.toBe(false)
  })

  it('falls back when navigator.clipboard is missing, as on any http origin', async () => {
    // The real bug: the old code called navigator.clipboard?.writeText(), which
    // is undefined outside a secure context — so it copied nothing and still
    // toasted "copied".
    setClipboard(undefined)
    const exec = vi.fn().mockReturnValue(true)
    document.execCommand = exec

    await expect(copyToClipboard('fallback-text')).resolves.toBe(true)
    expect(exec).toHaveBeenCalledWith('copy')
  })

  it('reports failure when the fallback is refused too', async () => {
    setClipboard(undefined)
    document.execCommand = vi.fn().mockReturnValue(false)

    await expect(copyToClipboard('x')).resolves.toBe(false)
  })

  it('cleans up its scratch node either way', async () => {
    setClipboard(undefined)
    document.execCommand = vi.fn().mockReturnValue(true)

    await copyToClipboard('x')

    expect(document.querySelectorAll('textarea')).toHaveLength(0)
  })

  it('cleans up even when execCommand throws', async () => {
    setClipboard(undefined)
    document.execCommand = vi.fn().mockImplementation(() => {
      throw new Error('boom')
    })

    await expect(copyToClipboard('x')).resolves.toBe(false)
    expect(document.querySelectorAll('textarea')).toHaveLength(0)
  })
})
