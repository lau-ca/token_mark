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
import { t } from 'i18next'

import { API_KEY_STATUSES } from '@/features/keys/constants'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { quotaUnitsToDollars } from '@/lib/format'

import type { KeyUsageExportReport } from './key-usage-export'

interface DownloadKeyUsageWorkbookOptions {
  report: KeyUsageExportReport
  username: string
  fileName: string
}

const COLORS = {
  navy: '18324A',
  navyLight: '244C6B',
  teal: '19A58C',
  tealLight: 'E7F7F3',
  blueLight: 'EAF2F8',
  slate: '5F6B76',
  border: 'D7DEE5',
  white: 'FFFFFF',
  rowAlt: 'F7F9FB',
  success: 'DFF4EA',
  warning: 'FFF3D6',
  danger: 'FCE8E6',
} as const

const INTEGER_FORMAT = '#,##0;[Red](#,##0);-'
const PERCENT_FORMAT = '0.0%;[Red](0.0%);-'
const DATE_FORMAT = 'yyyy-mm-dd hh:mm:ss'

function toExcelLocalDate(timestamp: number) {
  const date = new Date(timestamp * 1000)
  return new Date(
    Date.UTC(
      date.getFullYear(),
      date.getMonth(),
      date.getDate(),
      date.getHours(),
      date.getMinutes(),
      date.getSeconds()
    )
  )
}

function applyTableHeader(row: import('exceljs').Row) {
  row.height = 25
  row.eachCell((cell) => {
    cell.font = {
      name: 'Arial',
      size: 10,
      bold: true,
      color: { argb: COLORS.white },
    }
    cell.fill = {
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: COLORS.navy },
    }
    cell.alignment = { vertical: 'middle', horizontal: 'center' }
    cell.border = {
      top: { style: 'thin', color: { argb: COLORS.border } },
      left: { style: 'thin', color: { argb: COLORS.border } },
      bottom: { style: 'thin', color: { argb: COLORS.border } },
      right: { style: 'thin', color: { argb: COLORS.border } },
    }
  })
}

function applyDataRow(row: import('exceljs').Row, alternate: boolean) {
  row.height = 22
  row.eachCell({ includeEmpty: true }, (cell) => {
    cell.font = { name: 'Arial', size: 10, color: { argb: COLORS.navy } }
    cell.alignment = { vertical: 'middle' }
    cell.border = {
      bottom: { style: 'hair', color: { argb: COLORS.border } },
    }
    if (alternate) {
      cell.fill = {
        type: 'pattern',
        pattern: 'solid',
        fgColor: { argb: COLORS.rowAlt },
      }
    }
  })
}

function applySheetTitle(
  sheet: import('exceljs').Worksheet,
  title: string,
  subtitle: string,
  lastColumn: number
) {
  sheet.mergeCells(1, 1, 2, lastColumn)
  const titleCell = sheet.getCell(1, 1)
  titleCell.value = title
  titleCell.font = {
    name: 'Arial',
    size: 20,
    bold: true,
    color: { argb: COLORS.white },
  }
  titleCell.fill = {
    type: 'pattern',
    pattern: 'solid',
    fgColor: { argb: COLORS.navy },
  }
  titleCell.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 }
  sheet.getRow(1).height = 28
  sheet.getRow(2).height = 20

  sheet.mergeCells(3, 1, 3, lastColumn)
  const subtitleCell = sheet.getCell(3, 1)
  subtitleCell.value = subtitle
  subtitleCell.font = { name: 'Arial', size: 10, color: { argb: COLORS.slate } }
  subtitleCell.alignment = { vertical: 'middle', horizontal: 'left', indent: 1 }
  sheet.getRow(3).height = 23
}

function getAmountFormat() {
  const { meta } = getCurrencyDisplay()
  if (meta.kind === 'tokens') return INTEGER_FORMAT
  const symbol = meta.symbol.replaceAll('"', '')
  return `"${symbol}"#,##0.00;[Red]("${symbol}"#,##0.00);-`
}

function getTokenStatusLabel(status: number, deleted: boolean) {
  if (deleted) return t('Deleted')
  const config = API_KEY_STATUSES[status]
  return config ? t(config.label) : t('Unknown')
}

function styleStatusCell(
  cell: import('exceljs').Cell,
  status: number,
  deleted: boolean
) {
  let color: string = COLORS.warning
  if (deleted || status === 4) color = COLORS.danger
  if (status === 1 && !deleted) color = COLORS.success
  cell.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: color } }
  cell.font = {
    name: 'Arial',
    size: 10,
    bold: true,
    color: { argb: COLORS.navy },
  }
  cell.alignment = { vertical: 'middle', horizontal: 'center' }
}

function addKeySummarySheet(
  workbook: import('exceljs').Workbook,
  report: KeyUsageExportReport
) {
  const sheet = workbook.addWorksheet(t('Key Summary'), {
    views: [{ state: 'frozen', ySplit: 5 }],
    properties: { defaultRowHeight: 21 },
  })
  const amountLabel = `${t('Consumption Amount')} (${getCurrencyLabel()})`
  applySheetTitle(
    sheet,
    t('Key Usage Summary'),
    t('All API keys in the selected period, including keys with zero usage.'),
    11
  )
  sheet.getRow(5).values = [
    t('Key ID'),
    t('API Key'),
    t('Masked Key'),
    t('Status'),
    t('Requests'),
    t('Total Tokens'),
    amountLabel,
    t('Share'),
    t('Models Used'),
    t('Last Used'),
    t('Raw Quota'),
  ]
  applyTableHeader(sheet.getRow(5))

  const firstDataRow = 6
  const lastDataRow = Math.max(
    firstDataRow,
    firstDataRow + report.keys.length - 1
  )
  for (const [index, item] of report.keys.entries()) {
    const rowNumber = firstDataRow + index
    const row = sheet.getRow(rowNumber)
    row.values = [
      item.token_id,
      item.token_name,
      item.masked_key || `ID ${item.token_id}`,
      getTokenStatusLabel(item.token_status, item.deleted),
      item.request_count,
      item.total_tokens,
      quotaUnitsToDollars(item.quota),
      {
        formula: `IFERROR(G${rowNumber}/SUM($G$${firstDataRow}:$G$${lastDataRow}),0)`,
        result: item.share,
      },
      item.model_count,
      item.last_used_at > 0 ? toExcelLocalDate(item.last_used_at) : null,
      item.quota,
    ]
    applyDataRow(row, index % 2 === 1)
    styleStatusCell(row.getCell(4), item.token_status, item.deleted)
    for (const column of [1, 5, 6, 9, 11]) {
      row.getCell(column).numFmt = INTEGER_FORMAT
      row.getCell(column).alignment = {
        vertical: 'middle',
        horizontal: 'right',
      }
    }
    row.getCell(7).numFmt = getAmountFormat()
    row.getCell(7).alignment = { vertical: 'middle', horizontal: 'right' }
    row.getCell(8).numFmt = PERCENT_FORMAT
    row.getCell(8).alignment = { vertical: 'middle', horizontal: 'right' }
    row.getCell(10).numFmt = DATE_FORMAT
  }

  if (report.keys.length === 0) {
    sheet.mergeCells(firstDataRow, 1, firstDataRow, 11)
    const emptyCell = sheet.getCell(firstDataRow, 1)
    emptyCell.value = t('No data for the selected period.')
    emptyCell.alignment = { vertical: 'middle', horizontal: 'center' }
    emptyCell.font = {
      name: 'Arial',
      size: 10,
      italic: true,
      color: { argb: COLORS.slate },
    }
    sheet.getRow(firstDataRow).height = 34
  }

  sheet.autoFilter = { from: 'A5', to: `K${lastDataRow}` }
  sheet.columns = [
    { width: 10 },
    { width: 24 },
    { width: 24 },
    { width: 13 },
    { width: 13 },
    { width: 16 },
    { width: 20 },
    { width: 12 },
    { width: 14 },
    { width: 21 },
    { width: 16 },
  ]
  sheet.pageSetup = {
    orientation: 'landscape',
    fitToPage: true,
    fitToWidth: 1,
    fitToHeight: 0,
  }
  sheet.headerFooter.oddFooter = `&L${t('Key Usage Report')}&R${t('Page')} &P / &N`
}

function addModelDetailsSheet(
  workbook: import('exceljs').Workbook,
  report: KeyUsageExportReport
) {
  const sheet = workbook.addWorksheet(t('Model Details'), {
    views: [{ state: 'frozen', ySplit: 5 }],
    properties: { defaultRowHeight: 21 },
  })
  const amountLabel = `${t('Consumption Amount')} (${getCurrencyLabel()})`
  applySheetTitle(
    sheet,
    t('Key Model Details'),
    t('Usage is grouped by API key and model for the selected period.'),
    10
  )
  sheet.getRow(5).values = [
    t('Key ID'),
    t('API Key'),
    t('Masked Key'),
    t('Model'),
    t('Requests'),
    t('Total Tokens'),
    amountLabel,
    t('Key Share'),
    t('Last Used'),
    t('Raw Quota'),
  ]
  applyTableHeader(sheet.getRow(5))

  const firstDataRow = 6
  const lastDataRow = Math.max(
    firstDataRow,
    firstDataRow + report.models.length - 1
  )
  for (const [index, item] of report.models.entries()) {
    const rowNumber = firstDataRow + index
    const row = sheet.getRow(rowNumber)
    row.values = [
      item.token_id,
      item.token_name,
      item.masked_key || `ID ${item.token_id}`,
      item.model_name,
      item.request_count,
      item.total_tokens,
      quotaUnitsToDollars(item.quota),
      {
        formula: `IFERROR(G${rowNumber}/SUMIF($A$${firstDataRow}:$A$${lastDataRow},A${rowNumber},$G$${firstDataRow}:$G$${lastDataRow}),0)`,
        result: item.key_share,
      },
      item.last_used_at > 0 ? toExcelLocalDate(item.last_used_at) : null,
      item.quota,
    ]
    applyDataRow(row, index % 2 === 1)
    for (const column of [1, 5, 6, 10]) {
      row.getCell(column).numFmt = INTEGER_FORMAT
      row.getCell(column).alignment = {
        vertical: 'middle',
        horizontal: 'right',
      }
    }
    row.getCell(7).numFmt = getAmountFormat()
    row.getCell(7).alignment = { vertical: 'middle', horizontal: 'right' }
    row.getCell(8).numFmt = PERCENT_FORMAT
    row.getCell(8).alignment = { vertical: 'middle', horizontal: 'right' }
    row.getCell(9).numFmt = DATE_FORMAT
  }

  if (report.models.length === 0) {
    sheet.mergeCells(firstDataRow, 1, firstDataRow, 10)
    const emptyCell = sheet.getCell(firstDataRow, 1)
    emptyCell.value = t('No model usage data for the selected period.')
    emptyCell.alignment = { vertical: 'middle', horizontal: 'center' }
    emptyCell.font = {
      name: 'Arial',
      size: 10,
      italic: true,
      color: { argb: COLORS.slate },
    }
    sheet.getRow(firstDataRow).height = 34
  }

  sheet.autoFilter = { from: 'A5', to: `J${lastDataRow}` }
  sheet.columns = [
    { width: 10 },
    { width: 24 },
    { width: 24 },
    { width: 28 },
    { width: 13 },
    { width: 16 },
    { width: 20 },
    { width: 13 },
    { width: 21 },
    { width: 16 },
  ]
  sheet.pageSetup = {
    orientation: 'landscape',
    fitToPage: true,
    fitToWidth: 1,
    fitToHeight: 0,
  }
  sheet.headerFooter.oddFooter = `&L${t('Key Usage Report')}&R${t('Page')} &P / &N`
}

function addOverviewSheet(
  workbook: import('exceljs').Workbook,
  report: KeyUsageExportReport,
  username: string,
  keySheetName: string,
  firstKeyRow: number,
  lastKeyRow: number
) {
  const sheet = workbook.addWorksheet(t('Data Overview'), {
    views: [{ state: 'frozen', ySplit: 4 }],
    properties: { defaultRowHeight: 22 },
  })
  applySheetTitle(
    sheet,
    t('Key Usage Report'),
    t('Professional API key consumption and model usage report.'),
    8
  )

  const info = [
    [t('Account'), username],
    [
      t('Statistics Period'),
      `${new Date(report.start_timestamp * 1000).toLocaleString()} - ${new Date(report.end_timestamp * 1000).toLocaleString()}`,
    ],
    [t('Generated At'), new Date(report.generated_at * 1000).toLocaleString()],
    [t('Currency'), getCurrencyLabel()],
  ]
  info.forEach(([label, value], index) => {
    const row = sheet.getRow(5 + index)
    row.getCell(1).value = label
    row.getCell(1).font = {
      name: 'Arial',
      size: 10,
      bold: true,
      color: { argb: COLORS.slate },
    }
    sheet.mergeCells(5 + index, 2, 5 + index, 8)
    row.getCell(2).value = value
    row.getCell(2).font = {
      name: 'Arial',
      size: 10,
      color: { argb: COLORS.navy },
    }
  })

  const safeSheetName = keySheetName.replaceAll("'", "''")
  const metrics = [
    {
      label: t('Active Keys'),
      formula: `COUNTIF('${safeSheetName}'!E${firstKeyRow}:E${lastKeyRow},">0")`,
      result: report.totals.active_keys,
      format: INTEGER_FORMAT,
    },
    {
      label: t('Models Used'),
      result: report.totals.model_count,
      format: INTEGER_FORMAT,
    },
    {
      label: t('Requests'),
      formula: `SUM('${safeSheetName}'!E${firstKeyRow}:E${lastKeyRow})`,
      result: report.totals.request_count,
      format: INTEGER_FORMAT,
    },
    {
      label: t('Total Tokens'),
      formula: `SUM('${safeSheetName}'!F${firstKeyRow}:F${lastKeyRow})`,
      result: report.totals.total_tokens,
      format: INTEGER_FORMAT,
    },
    {
      label: t('Consumption Amount'),
      formula: `SUM('${safeSheetName}'!G${firstKeyRow}:G${lastKeyRow})`,
      result: quotaUnitsToDollars(report.totals.quota),
      format: getAmountFormat(),
    },
    {
      label: t('Total Keys'),
      result: report.keys.length,
      format: INTEGER_FORMAT,
    },
  ]

  metrics.forEach((metric, index) => {
    const metricRow = index < 3 ? 10 : 13
    const metricColumn = (index % 3) * 3 + 1
    sheet.mergeCells(metricRow, metricColumn, metricRow, metricColumn + 1)
    sheet.mergeCells(
      metricRow + 1,
      metricColumn,
      metricRow + 2,
      metricColumn + 1
    )
    const labelCell = sheet.getCell(metricRow, metricColumn)
    labelCell.value = metric.label
    labelCell.font = {
      name: 'Arial',
      size: 10,
      bold: true,
      color: { argb: COLORS.slate },
    }
    labelCell.fill = {
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: COLORS.blueLight },
    }
    labelCell.alignment = { vertical: 'middle', horizontal: 'center' }
    const valueCell = sheet.getCell(metricRow + 1, metricColumn)
    valueCell.value = metric.formula
      ? { formula: metric.formula, result: metric.result }
      : metric.result
    valueCell.font = {
      name: 'Arial',
      size: 18,
      bold: true,
      color: { argb: COLORS.navy },
    }
    valueCell.fill = {
      type: 'pattern',
      pattern: 'solid',
      fgColor: { argb: COLORS.tealLight },
    }
    valueCell.alignment = { vertical: 'middle', horizontal: 'center' }
    valueCell.numFmt = metric.format
    for (let row = metricRow; row <= metricRow + 2; row += 1) {
      for (let column = metricColumn; column <= metricColumn + 1; column += 1) {
        sheet.getCell(row, column).border = {
          top: { style: 'thin', color: { argb: COLORS.border } },
          left: { style: 'thin', color: { argb: COLORS.border } },
          bottom: { style: 'thin', color: { argb: COLORS.border } },
          right: { style: 'thin', color: { argb: COLORS.border } },
        }
      }
    }
  })

  sheet.mergeCells('A17:H17')
  sheet.getCell('A17').value = t('Report Notes')
  sheet.getCell('A17').font = {
    name: 'Arial',
    size: 11,
    bold: true,
    color: { argb: COLORS.white },
  }
  sheet.getCell('A17').fill = {
    type: 'pattern',
    pattern: 'solid',
    fgColor: { argb: COLORS.navyLight },
  }
  sheet.getCell('A17').alignment = {
    vertical: 'middle',
    horizontal: 'left',
    indent: 1,
  }
  const notes = [
    t('The report only contains data for the currently signed-in account.'),
    t('API keys are masked and full key values are never included.'),
    t(
      'Only successful consumption records in the selected period are aggregated.'
    ),
    t('Zero values are displayed as dashes for readability.'),
  ]
  notes.forEach((note, index) => {
    sheet.mergeCells(18 + index, 1, 18 + index, 8)
    const cell = sheet.getCell(18 + index, 1)
    cell.value = `• ${note}`
    cell.font = { name: 'Arial', size: 10, color: { argb: COLORS.slate } }
    cell.alignment = {
      vertical: 'middle',
      horizontal: 'left',
      indent: 1,
      wrapText: true,
    }
  })

  sheet.columns = Array.from({ length: 8 }, () => ({ width: 16 }))
  sheet.pageSetup = {
    orientation: 'landscape',
    fitToPage: true,
    fitToWidth: 1,
    fitToHeight: 1,
  }
  sheet.headerFooter.oddFooter = `&L${t('Key Usage Report')}&R${t('Page')} &P / &N`
}

export async function buildKeyUsageWorkbook(
  options: DownloadKeyUsageWorkbookOptions
) {
  const { Workbook } = await import('exceljs')
  const workbook = new Workbook()
  workbook.creator = 'FriModel'
  workbook.created = new Date(options.report.generated_at * 1000)
  workbook.modified = new Date()
  workbook.calcProperties.fullCalcOnLoad = true

  const firstKeyRow = 6
  const lastKeyRow = Math.max(
    firstKeyRow,
    firstKeyRow + options.report.keys.length - 1
  )
  addOverviewSheet(
    workbook,
    options.report,
    options.username,
    t('Key Summary'),
    firstKeyRow,
    lastKeyRow
  )
  addKeySummarySheet(workbook, options.report)
  addModelDetailsSheet(workbook, options.report)

  const buffer = await workbook.xlsx.writeBuffer()
  return new Uint8Array(buffer)
}

export async function downloadKeyUsageWorkbook(
  options: DownloadKeyUsageWorkbookOptions
) {
  const bytes = await buildKeyUsageWorkbook(options)
  const blob = new Blob([bytes], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  try {
    anchor.href = url
    anchor.download = options.fileName
    document.body.appendChild(anchor)
    anchor.click()
  } finally {
    anchor.remove()
    URL.revokeObjectURL(url)
  }
}
