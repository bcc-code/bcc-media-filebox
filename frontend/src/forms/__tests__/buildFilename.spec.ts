import { describe, it, expect } from 'vitest'
import { buildFilename, getForm } from '../index'

// Mirrors TestBuildFilenameTransliterates in internal/forms/forms_test.go.
describe('buildFilename', () => {
  it('folds accented and Nordic letters instead of dropping them', () => {
    const form = getForm('masters')!
    expect(buildFilename(form, { project: 'PROJ', title: '- Norwegian: å ø æ' }, '.mov')).toBe(
      'PROJ_-_Norwegian_a_o_ae.mov',
    )
  })
})
