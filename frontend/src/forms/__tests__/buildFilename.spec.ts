import { describe, it, expect } from 'vitest'
import { buildFilename, expectedFilename, fileExt, getForm } from '../index'

// Mirrors TestBuildFilenameTransliterates in internal/forms/forms_test.go.
describe('buildFilename', () => {
  it('folds accented and Nordic letters instead of dropping them', () => {
    const form = getForm('masters')!
    expect(buildFilename(form, { project: 'PROJ', title: '- Norwegian: å ø æ' }, '.mov')).toBe(
      'PROJ_-_Norwegian_a_o_ae.mov',
    )
  })
})

describe('expectedFilename', () => {
  const file = (name: string) => new File(['x'], name)

  it('sanitizes the original name for targets without a form', () => {
    expect(expectedFilename(file('Åse møte (v2).MOV'), null, {})).toEqual({
      name: 'Ase_mote__v2_.MOV',
      error: null,
    })
  })

  it('builds the form name and keeps the sanitized extension', () => {
    const form = getForm('masters')!
    const values = { project: 'PROJ', season: 'S1', episode: 'E2', title: 'cold open' }
    expect(expectedFilename(file('raw take.v1.mov'), form, values).name).toBe(
      'PROJ_S1_E2_cold_open.mov',
    )
    expect(expectedFilename(file('noext'), form, values).name).toBe('PROJ_S1_E2_cold_open')
  })

  it('reports names the server would reject', () => {
    expect(expectedFilename(file('..'), null, {}).error).toBe('Invalid filename')
  })
})

describe('fileExt', () => {
  it('matches Go filepath.Ext', () => {
    expect(fileExt('a.tar.gz')).toBe('.gz')
    expect(fileExt('a')).toBe('')
    expect(fileExt('_env')).toBe('')
  })
})
