import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getPlaygroundParameterValidationError } from './playground-parameter-validation'

describe('playground media parameter validation', () => {
  const durationParameter = {
    key: 'duration',
    type: 'number',
    required: true,
    min: 4,
    max: 15,
  } as const

  test('distinguishes required, range, and valid values', () => {
    assert.equal(
      getPlaygroundParameterValidationError(durationParameter, undefined),
      'required'
    )
    assert.equal(
      getPlaygroundParameterValidationError(durationParameter, 1),
      'range'
    )
    assert.equal(
      getPlaygroundParameterValidationError(durationParameter, 16),
      'range'
    )
    assert.equal(
      getPlaygroundParameterValidationError(durationParameter, 4),
      null
    )
    assert.equal(
      getPlaygroundParameterValidationError(durationParameter, 15),
      null
    )
  })
})
