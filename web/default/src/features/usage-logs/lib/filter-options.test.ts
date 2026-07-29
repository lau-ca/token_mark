import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getModelsForLogGroup,
  mergeLogFilterModels,
  type LogFilterGroup,
  type LogFilterModel,
} from './filter-options'

const groups: LogFilterGroup[] = [
  { name: 'default' },
  { name: 'image_web' },
  {
    name: 'image_stable',
    composite: true,
    publicModel: 'gpt-image-2',
  },
]

const models: LogFilterModel[] = [
  { name: 'gpt-4.1', groups: ['all'] },
  { name: 'gpt-image-2', groups: ['default'] },
  { name: 'gpt-image-2-w', groups: ['image_web'] },
]

describe('usage log filter options', () => {
  test('keeps all models when no group or auto is selected', () => {
    assert.deepEqual(getModelsForLogGroup(models, groups, ''), models)
    assert.deepEqual(getModelsForLogGroup(models, groups, 'auto'), models)
  })

  test('filters physical groups while retaining globally enabled models', () => {
    assert.deepEqual(getModelsForLogGroup(models, groups, 'image_web'), [
      models[0],
      models[2],
    ])
  })

  test('limits a composite group to its public model', () => {
    assert.deepEqual(getModelsForLogGroup(models, groups, 'image_stable'), [
      models[1],
    ])
  })

  test('adds composite public models without duplicating pricing models', () => {
    assert.deepEqual(mergeLogFilterModels(models, groups), [
      { name: 'gpt-4.1', groups: ['all'] },
      { name: 'gpt-image-2', groups: ['default', 'image_stable'] },
      { name: 'gpt-image-2-w', groups: ['image_web'] },
    ])
  })
})
