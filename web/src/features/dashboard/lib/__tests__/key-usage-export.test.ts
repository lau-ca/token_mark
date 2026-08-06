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

import type { KeyUsageExportData } from '../../types'
import {
  buildKeyUsageExportFileName,
  buildKeyUsageExportReport,
  sanitizeExportFileSegment,
} from '../key-usage-export'

const data: KeyUsageExportData = {
  generated_at: 1_786_000_000,
  start_timestamp: 1_785_614_400,
  end_timestamp: 1_786_305_599,
  keys: [
    {
      token_id: 12,
      token_name: 'backup',
      masked_key: 'back****key',
      token_status: 2,
      request_count: 0,
      prompt_tokens: 0,
      completion_tokens: 0,
      total_tokens: 0,
      quota: 0,
      model_count: 0,
      last_used_at: 0,
      deleted: false,
    },
    {
      token_id: 11,
      token_name: 'primary',
      masked_key: 'prim****key',
      token_status: 1,
      request_count: 3,
      prompt_tokens: 230,
      completion_tokens: 70,
      total_tokens: 300,
      quota: 600,
      model_count: 2,
      last_used_at: 1_785_700_000,
      deleted: false,
    },
    {
      token_id: 13,
      token_name: 'old-name',
      masked_key: '',
      token_status: 0,
      request_count: 1,
      prompt_tokens: 30,
      completion_tokens: 5,
      total_tokens: 35,
      quota: 200,
      model_count: 1,
      last_used_at: 1_785_800_000,
      deleted: true,
    },
  ],
  models: [
    {
      token_id: 11,
      token_name: 'primary',
      masked_key: 'prim****key',
      model_name: 'claude-opus',
      request_count: 1,
      prompt_tokens: 80,
      completion_tokens: 20,
      total_tokens: 100,
      quota: 200,
      last_used_at: 1_785_650_000,
      deleted: false,
    },
    {
      token_id: 13,
      token_name: 'old-name',
      masked_key: '',
      model_name: '',
      request_count: 1,
      prompt_tokens: 30,
      completion_tokens: 5,
      total_tokens: 35,
      quota: 200,
      last_used_at: 1_785_800_000,
      deleted: true,
    },
    {
      token_id: 11,
      token_name: 'primary',
      masked_key: 'prim****key',
      model_name: 'gpt-5',
      request_count: 2,
      prompt_tokens: 150,
      completion_tokens: 50,
      total_tokens: 200,
      quota: 400,
      last_used_at: 1_785_700_000,
      deleted: false,
    },
  ],
}

describe('Key usage export report', () => {
  test('normalizes names, totals, shares and report ordering', () => {
    const report = buildKeyUsageExportReport(data, {
      deletedKey: (id) => `Deleted (${id})`,
      unnamedKey: (id) => `Key ${id}`,
      unknownModel: 'Unknown model',
    })

    assert.deepEqual(
      report.keys.map((item) => item.token_id),
      [11, 13, 12]
    )
    assert.equal(report.keys[0].share, 0.75)
    assert.equal(report.keys[1].token_name, 'Deleted (13)')
    assert.deepEqual(
      report.models.map((item) => [item.token_id, item.model_name]),
      [
        [11, 'gpt-5'],
        [11, 'claude-opus'],
        [13, 'Unknown model'],
      ]
    )
    assert.equal(report.models[0].key_share, 2 / 3)
    assert.deepEqual(report.totals, {
      active_keys: 2,
      model_count: 3,
      request_count: 4,
      prompt_tokens: 260,
      completion_tokens: 75,
      total_tokens: 335,
      quota: 800,
    })
  })

  test('returns zero shares for an empty report', () => {
    const report = buildKeyUsageExportReport(
      { ...data, keys: [], models: [] },
      {
        deletedKey: (id) => `Deleted (${id})`,
        unnamedKey: (id) => `Key ${id}`,
        unknownModel: 'Unknown model',
      }
    )

    assert.equal(report.keys.length, 0)
    assert.equal(report.models.length, 0)
    assert.equal(report.totals.quota, 0)
  })

  test('sanitizes file segments and includes the selected dates', () => {
    assert.equal(sanitizeExportFileSegment(' fri day:* '), 'fri-day_')
    assert.match(
      buildKeyUsageExportFileName(
        'Key Usage Report',
        'fri/day',
        data.start_timestamp,
        data.end_timestamp
      ),
      /^Key-Usage-Report_fri_day_\d{4}-\d{2}-\d{2}_\d{4}-\d{2}-\d{2}\.xlsx$/
    )
  })
})
