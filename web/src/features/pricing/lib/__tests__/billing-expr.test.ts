/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { parseTiersFromExpr } from '../billing-expr'

const seedanceExpression = `task_tokens(
  param("resolution") == "4k"
    ? tier("4k", c * (param("has_reference_video") ? 16 : 26))
    : param("resolution") == "1080p"
      ? tier("1080p", c * (param("has_reference_video") ? 31 : 51))
      : tier("480p_720p", c * (param("has_reference_video") ? 28 : 46))
)`

describe('task token billing expression parsing', () => {
  test('returns every resolution and reference-video price combination', () => {
    assert.deepEqual(parseTiersFromExpr(seedanceExpression), [
      {
        label: '4k',
        conditions: [],
        outputPrice: 26,
        taskResolution: '4k',
        hasReferenceVideo: false,
        isTaskTokenPrice: true,
      },
      {
        label: '4k',
        conditions: [],
        outputPrice: 16,
        taskResolution: '4k',
        hasReferenceVideo: true,
        isTaskTokenPrice: true,
      },
      {
        label: '1080p',
        conditions: [],
        outputPrice: 51,
        taskResolution: '1080p',
        hasReferenceVideo: false,
        isTaskTokenPrice: true,
      },
      {
        label: '1080p',
        conditions: [],
        outputPrice: 31,
        taskResolution: '1080p',
        hasReferenceVideo: true,
        isTaskTokenPrice: true,
      },
      {
        label: '480p_720p',
        conditions: [],
        outputPrice: 46,
        taskResolution: '480p / 720p',
        hasReferenceVideo: false,
        isTaskTokenPrice: true,
      },
      {
        label: '480p_720p',
        conditions: [],
        outputPrice: 28,
        taskResolution: '480p / 720p',
        hasReferenceVideo: true,
        isTaskTokenPrice: true,
      },
    ])
  })

  test('keeps unsupported task token expressions on the raw-expression fallback', () => {
    assert.deepEqual(parseTiersFromExpr('task_tokens(c * 46)'), [])
  })
})
