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

import { Workbook } from 'exceljs'
import i18next from 'i18next'

import type { KeyUsageExportReport } from '../key-usage-export'
import { buildKeyUsageWorkbook } from '../key-usage-workbook'

const report: KeyUsageExportReport = {
  generated_at: 1_786_000_000,
  start_timestamp: 1_785_614_400,
  end_timestamp: 1_786_305_599,
  keys: [
    {
      token_id: 11,
      token_name: 'primary',
      masked_key: 'prim****key',
      token_status: 1,
      request_count: 2,
      total_tokens: 200,
      quota: 500_000,
      model_count: 1,
      last_used_at: 1_785_700_000,
      deleted: false,
      share: 1,
    },
  ],
  models: [
    {
      token_id: 11,
      token_name: 'primary',
      masked_key: 'prim****key',
      model_name: 'gpt-5',
      request_count: 2,
      total_tokens: 200,
      quota: 500_000,
      last_used_at: 1_785_700_000,
      deleted: false,
      key_share: 1,
    },
  ],
  totals: {
    active_keys: 1,
    model_count: 1,
    request_count: 2,
    total_tokens: 200,
    quota: 500_000,
  },
}

describe('Key usage workbook', () => {
  test('creates styled overview, Key summary and model detail sheets', async () => {
    await i18next.init({ lng: 'en', resources: { en: { translation: {} } } })
    const bytes = await buildKeyUsageWorkbook({
      report,
      username: 'friday',
      fileName: 'report.xlsx',
    })
    const workbook = new Workbook()
    await workbook.xlsx.load(
      bytes as unknown as Parameters<typeof workbook.xlsx.load>[0]
    )

    assert.deepEqual(
      workbook.worksheets.map((sheet) => sheet.name),
      ['Data Overview', 'Key Summary', 'Model Details']
    )
    const overview = workbook.getWorksheet('Data Overview')
    const keySummary = workbook.getWorksheet('Key Summary')
    const modelDetails = workbook.getWorksheet('Model Details')
    assert.ok(overview)
    assert.ok(keySummary)
    assert.ok(modelDetails)
    const [keySummaryView] = keySummary.views
    assert.ok(keySummaryView && keySummaryView.state === 'frozen')
    assert.equal(keySummaryView.ySplit, 5)
    assert.equal(keySummary.autoFilter?.toString(), 'A5:K6')
    assert.equal(keySummary.getCell('F6').value, 200)
    assert.deepEqual(keySummary.getCell('H6').value, {
      formula: 'IFERROR(G6/SUM($G$6:$G$6),0)',
      result: 1,
    })
    assert.equal(modelDetails.getCell('F6').value, 200)
    assert.equal(keySummary.getCell('A5').font.bold, true)
    assert.equal(overview.getCell('A1').value, 'Key Usage Report')
  })
})
