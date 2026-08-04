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
/**
 * Billing expression parsing utilities.
 *
 * Parses the dynamic billing expression format so that the pricing breakdown
 * UI can be rendered from the same backend expressions.
 *
 * The grammar is intentionally narrow: we only support the shapes that the
 * server emits (tiered pricing + request-rule conditional multipliers), so
 * the regular expressions are exact rather than tolerant of arbitrary
 * expression syntax.
 */

// ---------------------------------------------------------------------------
// Variable registry
// ---------------------------------------------------------------------------

export type BillingVar = {
  key: string
  field: string | null
  tierField: string | null
  label: string
  shortLabel: string
  side: 'input' | 'output' | 'condition'
  isBase?: boolean
  isConditionOnly?: boolean
  group?: string
}

export const BILLING_VARS: BillingVar[] = [
  {
    key: 'p',
    field: 'inputPrice',
    tierField: 'input_unit_cost',
    label: 'Input price',
    shortLabel: 'Input',
    side: 'input',
    isBase: true,
  },
  {
    key: 'c',
    field: 'outputPrice',
    tierField: 'output_unit_cost',
    label: 'Completion price',
    shortLabel: 'Output',
    side: 'output',
    isBase: true,
  },
  {
    key: 'len',
    field: null,
    tierField: null,
    label: 'Input length',
    shortLabel: 'Length',
    side: 'condition',
    isConditionOnly: true,
  },
  {
    key: 'cr',
    field: 'cacheReadPrice',
    tierField: 'cache_read_unit_cost',
    label: 'Cache read price',
    shortLabel: 'Cache Read',
    side: 'input',
    group: 'cache',
  },
  {
    key: 'cc',
    field: 'cacheCreatePrice',
    tierField: 'cache_create_unit_cost',
    label: 'Cache create price',
    shortLabel: 'Cache Write',
    side: 'input',
    group: 'cache',
  },
  {
    key: 'cc1h',
    field: 'cacheCreate1hPrice',
    tierField: 'cache_create_1h_unit_cost',
    label: 'Cache create (1h) price',
    shortLabel: 'Cache Write (1h)',
    side: 'input',
    group: 'cache',
  },
  {
    key: 'img',
    field: 'imagePrice',
    tierField: 'image_unit_cost',
    label: 'Image input price',
    shortLabel: 'Image In',
    side: 'input',
    group: 'media',
  },
  {
    key: 'img_o',
    field: 'imageOutputPrice',
    tierField: 'image_output_unit_cost',
    label: 'Image output price',
    shortLabel: 'Image Out',
    side: 'output',
    group: 'media',
  },
  {
    key: 'ai',
    field: 'audioInputPrice',
    tierField: 'audio_input_unit_cost',
    label: 'Audio input price',
    shortLabel: 'Audio In',
    side: 'input',
    group: 'media',
  },
  {
    key: 'ao',
    field: 'audioOutputPrice',
    tierField: 'audio_output_unit_cost',
    label: 'Audio output price',
    shortLabel: 'Audio Out',
    side: 'output',
    group: 'media',
  },
]

/** Vars that have real price fields (excludes condition-only vars like `len`) */
export const BILLING_PRICING_VARS: BillingVar[] = BILLING_VARS.filter(
  (v) => !v.isConditionOnly
)

/** Vars valid in tier conditions (`p`, `c`, `len`) */
export const BILLING_CONDITION_VARS: string[] = BILLING_VARS.filter(
  (v) => v.isBase || v.isConditionOnly
).map((v) => v.key)

const BILLING_VAR_KEY_TO_FIELD = Object.fromEntries(
  BILLING_PRICING_VARS.map((v) => [v.key, v.field as string])
) as Record<string, string>

export const BILLING_EXTRA_VARS: BillingVar[] = BILLING_VARS.filter(
  (v) => !v.isBase && !v.isConditionOnly
)

export const BILLING_CACHE_VAR_MAP = BILLING_EXTRA_VARS.map((v) => ({
  field: v.tierField as string,
  exprVar: v.key,
}))

const NUMERIC_LITERAL_REGEX = /^-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?$/
const BILLING_VAR_NAME_SET = new Set(
  BILLING_PRICING_VARS.map((variable) => variable.key)
)

// ---------------------------------------------------------------------------
// Request rule constants
// ---------------------------------------------------------------------------

export const SOURCE_PARAM = 'param'
export const SOURCE_HEADER = 'header'
export const SOURCE_TIME = 'time'

export const MATCH_EQ = 'eq'
export const MATCH_CONTAINS = 'contains'
export const MATCH_GT = 'gt'
export const MATCH_GTE = 'gte'
export const MATCH_LT = 'lt'
export const MATCH_LTE = 'lte'
export const MATCH_EXISTS = 'exists'
export const MATCH_RANGE = 'range'

export const TIME_FUNCS = ['hour', 'minute', 'weekday', 'month', 'day'] as const
export type TimeFunc = (typeof TIME_FUNCS)[number]

export const COMMON_TIMEZONES: { value: string; label: string }[] = [
  { value: 'Asia/Shanghai', label: 'UTC+8 Shanghai (Asia/Shanghai)' },
  { value: 'UTC', label: 'UTC' },
  { value: 'America/New_York', label: 'UTC-5 New York (America/New_York)' },
  {
    value: 'America/Los_Angeles',
    label: 'UTC-8 Los Angeles (America/Los_Angeles)',
  },
  { value: 'America/Chicago', label: 'UTC-6 Chicago (America/Chicago)' },
  { value: 'Europe/London', label: 'UTC+0 London (Europe/London)' },
  { value: 'Europe/Berlin', label: 'UTC+1 Berlin (Europe/Berlin)' },
  { value: 'Asia/Tokyo', label: 'UTC+9 Tokyo (Asia/Tokyo)' },
  { value: 'Asia/Singapore', label: 'UTC+8 Singapore (Asia/Singapore)' },
  { value: 'Asia/Seoul', label: 'UTC+9 Seoul (Asia/Seoul)' },
  { value: 'Australia/Sydney', label: 'UTC+10 Sydney (Australia/Sydney)' },
]

export type ParamHeaderCondition = {
  source: 'param' | 'header'
  path: string
  mode: string
  value: string
}

export type TimeCondition = {
  source: 'time'
  timeFunc: TimeFunc
  timezone: string
  mode: string
  value: string
  rangeStart: string
  rangeEnd: string
}

export type RequestCondition = TimeCondition | ParamHeaderCondition

export type RequestRuleGroup = {
  conditions: RequestCondition[]
  multiplier: string
}

export type TierCondition = {
  var: 'p' | 'c' | 'len'
  op: '<' | '<=' | '>' | '>='
  value: number
}

export type ParsedTier = {
  label: string
  conditions: TierCondition[]
  requestPrice?: number
  secondPrice?: number
  imageRequestPrice?: number
  [field: string]: unknown
}

export type TierUnitPrice = {
  unit: 'request' | 'second' | 'image'
  price: number
}

// ---------------------------------------------------------------------------
// Tier parser
// ---------------------------------------------------------------------------

function stripExprVersion(exprStr: string): { version: number; body: string } {
  if (!exprStr) return { version: 1, body: '' }
  const m = exprStr.match(/^v(\d+):([\s\S]*)$/)
  if (m) return { version: Number(m[1]), body: m[2] }
  return { version: 1, body: exprStr }
}

type ParsedFunctionCall = {
  start: number
  end: number
  args: string[]
}

function isIdentifierChar(char: string | undefined): boolean {
  return /[A-Za-z0-9_]/.test(char || '')
}

function findMatchingParen(expr: string, openingIndex: number): number {
  let depth = 0
  let quote = ''
  let escaped = false

  for (let index = openingIndex; index < expr.length; index += 1) {
    const char = expr[index]
    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = ''
      }
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      continue
    }
    if (char === '(') depth += 1
    if (char === ')') {
      depth -= 1
      if (depth === 0) return index
    }
  }
  return -1
}

function splitTopLevelArguments(expr: string): string[] {
  const parts: string[] = []
  let start = 0
  let depth = 0
  let quote = ''
  let escaped = false

  for (let index = 0; index < expr.length; index += 1) {
    const char = expr[index]
    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = ''
      }
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      continue
    }
    if (char === '(') depth += 1
    if (char === ')') depth -= 1
    if (char === ',' && depth === 0) {
      parts.push(expr.slice(start, index).trim())
      start = index + 1
    }
  }
  parts.push(expr.slice(start).trim())
  return parts
}

function findFunctionCalls(
  expr: string,
  functionName: string
): ParsedFunctionCall[] {
  const calls: ParsedFunctionCall[] = []
  let quote = ''
  let escaped = false

  for (let index = 0; index < expr.length; index += 1) {
    const char = expr[index]
    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = ''
      }
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      continue
    }
    if (!expr.startsWith(functionName, index)) continue

    const before = expr[index - 1]
    const afterName = expr[index + functionName.length]
    if (
      (before && /[\w$]/.test(before)) ||
      (afterName && /[\w$]/.test(afterName))
    ) {
      continue
    }

    let openingIndex = index + functionName.length
    while (/\s/.test(expr[openingIndex] || '')) openingIndex += 1
    if (expr[openingIndex] !== '(') continue

    const closingIndex = findMatchingParen(expr, openingIndex)
    if (closingIndex === -1) continue
    calls.push({
      start: index,
      end: closingIndex,
      args: splitTopLevelArguments(expr.slice(openingIndex + 1, closingIndex)),
    })
    index = closingIndex
  }
  return calls
}

function hasFullOuterParens(expr: string): boolean {
  return (
    expr.startsWith('(') &&
    expr.endsWith(')') &&
    findMatchingParen(expr, 0) === expr.length - 1
  )
}

function unwrapOuterParens(expr: string): string {
  let current = (expr || '').trim()
  while (hasFullOuterParens(current)) {
    current = current.slice(1, -1).trim()
  }
  return current
}

function parseStringLiteral(value: string): string | null {
  try {
    const parsed = JSON.parse(value.trim()) as unknown
    return typeof parsed === 'string' ? parsed : null
  } catch {
    return null
  }
}

function isDirectTierBranch(
  source: string,
  start: number,
  end: number
): boolean {
  let previousIndex = start - 1
  while (previousIndex >= 0 && /\s/.test(source[previousIndex])) {
    previousIndex -= 1
  }

  if (previousIndex >= 0) {
    const previousChar = source[previousIndex]
    if (!['?', ':', '('].includes(previousChar)) return false
    if (previousChar === '(') {
      let beforeParenthesis = previousIndex - 1
      while (beforeParenthesis >= 0 && /\s/.test(source[beforeParenthesis])) {
        beforeParenthesis -= 1
      }
      if (isIdentifierChar(source[beforeParenthesis])) return false
    }
  }

  let nextIndex = end + 1
  while (nextIndex < source.length && /\s/.test(source[nextIndex])) {
    nextIndex += 1
  }
  if (nextIndex >= source.length) return true
  return [':', ')'].includes(source[nextIndex])
}

export function isRequestDependentBillingExpr(exprStr: string): boolean {
  if (!exprStr) return false
  const { body } = stripExprVersion(exprStr)
  return ['per_request', 'param', 'header'].some(
    (functionName) => findFunctionCalls(body, functionName).length > 0
  )
}

function parseCompleteFunctionCall(
  expr: string,
  functionName: string
): ParsedFunctionCall | null {
  const body = unwrapOuterParens(expr)
  const calls = findFunctionCalls(body, functionName)
  if (
    calls.length !== 1 ||
    calls[0].start !== 0 ||
    calls[0].end !== body.length - 1
  ) {
    return null
  }
  return calls[0]
}

function parsePerRequestUnitPrice(bodyStr: string): TierUnitPrice | null {
  const factors = splitTopLevelMultiply(unwrapOuterParens(bodyStr))
  let price: number | null = null
  let hasDuration = false

  for (const factor of factors) {
    const perRequestCall = parseCompleteFunctionCall(factor, 'per_request')
    if (perRequestCall) {
      if (price !== null || perRequestCall.args.length !== 1) return null
      const amount = perRequestCall.args[0].trim()
      if (!NUMERIC_LITERAL_REGEX.test(amount)) return null
      price = Number(amount)
      if (!Number.isFinite(price)) return null
      continue
    }

    const paramCall = parseCompleteFunctionCall(factor, 'param')
    if (paramCall) {
      if (hasDuration || paramCall.args.length !== 1) return null
      hasDuration = parseStringLiteral(paramCall.args[0]) === 'duration'
      if (!hasDuration) return null
      continue
    }
    return null
  }

  if (price === null) return null
  if (factors.length === 1 && !hasDuration) {
    return { unit: 'request', price }
  }
  if (factors.length === 2 && hasDuration) {
    return { unit: 'second', price }
  }
  return null
}

function isNormalizedImageCountFactor(source: string): boolean {
  const body = unwrapOuterParens(source)
  const paramN = `param\\(\\s*["']n["']\\s*\\)`
  return new RegExp(
    `^${paramN}\\s*==\\s*nil\\s*\\|\\|\\s*${paramN}\\s*<=\\s*0\\s*\\?\\s*1\\s*:\\s*${paramN}$`
  ).test(body)
}

function parseImageRequestUnitPrice(bodyStr: string): TierUnitPrice | null {
  const factors = splitTopLevelMultiply(unwrapOuterParens(bodyStr))
  if (factors.length !== 2) return null

  const numericFactor = factors.find((factor) =>
    NUMERIC_LITERAL_REGEX.test(unwrapOuterParens(factor))
  )
  const quantityFactor = factors.find(isNormalizedImageCountFactor)
  if (!numericFactor || !quantityFactor) return null

  const expressionUnitPrice = Number(unwrapOuterParens(numericFactor))
  if (!Number.isFinite(expressionUnitPrice)) return null
  return { unit: 'image', price: expressionUnitPrice / 1_000_000 }
}

function splitTopLevelAddition(expr: string): string[] | null {
  const parts: string[] = []
  let start = 0
  let depth = 0

  for (let index = 0; index < expr.length; index += 1) {
    const char = expr[index]
    if (char === '(') {
      depth += 1
      continue
    }
    if (char === ')') {
      depth -= 1
      if (depth < 0) return null
      continue
    }
    if (depth !== 0 || char !== '+') continue

    let previousIndex = index - 1
    while (previousIndex >= 0 && /\s/.test(expr[previousIndex])) {
      previousIndex -= 1
    }
    const previousChar = expr[previousIndex]
    if (
      previousIndex < 0 ||
      ['*', '/', '+', '-', '('].includes(previousChar) ||
      previousChar === 'e' ||
      previousChar === 'E'
    ) {
      continue
    }

    parts.push(expr.slice(start, index).trim())
    start = index + 1
  }

  if (depth !== 0) return null
  parts.push(expr.slice(start).trim())
  return parts.every(Boolean) ? parts : null
}

function parseTokenTierBody(body: string): Record<string, number> | null {
  const terms = splitTopLevelAddition(body)
  if (!terms) return null

  const coefficients: Record<string, number> = {}
  for (const term of terms) {
    const factors = splitTopLevelMultiply(unwrapOuterParens(term))
    if (factors.length < 1 || factors.length > 2) return null

    const variableName = unwrapOuterParens(factors[0])
    if (!BILLING_VAR_NAME_SET.has(variableName)) return null
    if (variableName in coefficients) return null

    let coefficient = 1
    if (factors.length === 2) {
      const coefficientSource = unwrapOuterParens(factors[1])
      if (!NUMERIC_LITERAL_REGEX.test(coefficientSource)) return null
      coefficient = Number(coefficientSource)
      if (!Number.isFinite(coefficient)) return null
    }
    coefficients[variableName] = coefficient
  }

  const tier: Record<string, number> = {}
  for (const [variableName, field] of Object.entries(
    BILLING_VAR_KEY_TO_FIELD
  )) {
    tier[field] = coefficients[variableName] || 0
  }
  return tier
}

function parseTierBody(bodyStr: string): Record<string, number> | null {
  const unitPrice = parsePerRequestUnitPrice(bodyStr)
  if (unitPrice) {
    return unitPrice.unit === 'request'
      ? { requestPrice: unitPrice.price }
      : { secondPrice: unitPrice.price }
  }

  const imageUnitPrice = parseImageRequestUnitPrice(bodyStr)
  if (imageUnitPrice) {
    return { imageRequestPrice: imageUnitPrice.price }
  }

  const body = unwrapOuterParens(bodyStr)
  if (NUMERIC_LITERAL_REGEX.test(body)) {
    return { requestPrice: Number(body) / 1_000_000 }
  }
  return parseTokenTierBody(body)
}

const TRAILING_TIER_CONDITION_REGEX = new RegExp(
  `((?:(?:p|c|len)\\s*(?:<=|>=|<|>)\\s*${NUMERIC_LITERAL_REGEX.source.slice(1, -1)})` +
    `(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<=|>=|<|>)\\s*${NUMERIC_LITERAL_REGEX.source.slice(1, -1)})*)\\s*\\?\\s*$`
)

function parseTierConditions(prefix: string): TierCondition[] {
  const conditionMatch = prefix.match(TRAILING_TIER_CONDITION_REGEX)
  if (!conditionMatch) return []

  const conditions: TierCondition[] = []
  for (const conditionPart of conditionMatch[1].split(/\s*&&\s*/)) {
    const match = conditionPart
      .trim()
      .match(
        /^(p|c|len)\s*(<=|>=|<|>)\s*(-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)$/
      )
    if (!match) continue
    conditions.push({
      var: match[1] as TierCondition['var'],
      op: match[2] as TierCondition['op'],
      value: Number(match[3]),
    })
  }
  return conditions
}

export function parseTiersFromExpr(exprStr: string): ParsedTier[] {
  if (!exprStr) return []
  try {
    const { body } = stripExprVersion(exprStr)
    const calls = findFunctionCalls(body, 'tier')
    if (calls.some((call) => !isDirectTierBranch(body, call.start, call.end))) {
      return []
    }

    const tiers: ParsedTier[] = []
    for (const call of calls) {
      if (call.args.length !== 2) continue
      const label = parseStringLiteral(call.args[0])
      if (label === null) continue
      const tierBody = parseTierBody(call.args[1])
      if (tierBody === null) return []
      if (
        (Number.isFinite(tierBody.requestPrice) &&
          tierBody.requestPrice <= 0) ||
        (Number.isFinite(tierBody.secondPrice) && tierBody.secondPrice <= 0) ||
        (Number.isFinite(tierBody.imageRequestPrice) &&
          tierBody.imageRequestPrice <= 0)
      ) {
        continue
      }
      const tier = tierBody as ParsedTier
      tier.label = label
      tier.conditions = parseTierConditions(body.slice(0, call.start))
      tiers.push(tier)
    }
    return tiers
  } catch {
    return []
  }
}

export function getTierUnitPrice(
  tier: ParsedTier | null | undefined
): TierUnitPrice | null {
  if (!tier) return null
  const requestPrice = Number(tier.requestPrice)
  if (Number.isFinite(requestPrice) && requestPrice > 0) {
    return { unit: 'request', price: requestPrice }
  }
  const secondPrice = Number(tier.secondPrice)
  if (Number.isFinite(secondPrice) && secondPrice > 0) {
    return { unit: 'second', price: secondPrice }
  }
  const imageRequestPrice = Number(tier.imageRequestPrice)
  if (Number.isFinite(imageRequestPrice) && imageRequestPrice > 0) {
    return { unit: 'image', price: imageRequestPrice }
  }
  return null
}

export function normalizeTierLabel(label: string | undefined): string {
  if (!label) return ''
  return label
    .replaceAll(/<[=＝]?|≤|＜[=＝]?/g, '<')
    .replaceAll(/>[=＝]?|≥|＞[=＝]?/g, '>')
    .replaceAll(/\s+/g, '')
    .toLowerCase()
}

// ---------------------------------------------------------------------------
// Request rule parser
// ---------------------------------------------------------------------------

function splitTopLevelMultiply(expr: string): string[] {
  const parts: string[] = []
  let start = 0
  let depth = 0
  let quote = ''
  let escaped = false
  for (let index = 0; index < expr.length; index += 1) {
    const char = expr[index]
    if (quote) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === quote) {
        quote = ''
      }
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      continue
    }
    if (char === '(') depth += 1
    if (char === ')') depth -= 1
    if (depth === 0 && char === '*') {
      parts.push(expr.slice(start, index).trim())
      start = index + 1
    }
  }
  parts.push(expr.slice(start).trim())
  return parts.filter(Boolean)
}

function splitTopLevelAnd(expr: string): string[] {
  const parts: string[] = []
  let start = 0
  let depth = 0
  for (let i = 0; i < expr.length; i += 1) {
    const c = expr[i]
    if (c === '(') depth += 1
    if (c === ')') depth -= 1
    if (depth === 0 && expr.slice(i, i + 4) === ' && ') {
      parts.push(expr.slice(start, i).trim())
      start = i + 4
      i += 3
    }
  }
  parts.push(expr.slice(start).trim())
  return parts.filter(Boolean)
}

function parseExprLiteral(raw: string): string | null {
  const text = raw.trim()
  if (text === 'true' || text === 'false') return text
  if (NUMERIC_LITERAL_REGEX.test(text)) return text
  try {
    return JSON.parse(text) as string
  } catch {
    return null
  }
}

function tryParseTimeCondition(expr: string): RequestCondition | null {
  let m = expr.match(
    /^(hour|minute|weekday|month|day)\("([^"]+)"\) >= ([\d.eE+-]+) \|\| \1\("\2"\) < ([\d.eE+-]+)$/
  )
  if (m) {
    return {
      source: 'time',
      timeFunc: m[1] as TimeFunc,
      timezone: m[2],
      mode: MATCH_RANGE,
      value: '',
      rangeStart: m[3],
      rangeEnd: m[4],
    }
  }
  m = expr.match(
    /^\((hour|minute|weekday|month|day)\("([^"]+)"\) >= ([\d.eE+-]+) \|\| \1\("\2"\) < ([\d.eE+-]+)\)$/
  )
  if (m) {
    return {
      source: 'time',
      timeFunc: m[1] as TimeFunc,
      timezone: m[2],
      mode: MATCH_RANGE,
      value: '',
      rangeStart: m[3],
      rangeEnd: m[4],
    }
  }
  m = expr.match(
    /^(hour|minute|weekday|month|day)\("([^"]+)"\) (==|>=|<) ([\d.eE+-]+)$/
  )
  if (m) {
    const opMap: Record<string, string> = {
      '==': MATCH_EQ,
      '>=': MATCH_GTE,
      '<': MATCH_LT,
    }
    return {
      source: 'time',
      timeFunc: m[1] as TimeFunc,
      timezone: m[2],
      mode: opMap[m[3]] || MATCH_EQ,
      value: m[4],
      rangeStart: '',
      rangeEnd: '',
    }
  }
  return null
}

function tryParseRequestCondition(expr: string): RequestCondition | null {
  const tc = tryParseTimeCondition(expr)
  if (tc) return tc

  let m = expr.match(/^header\("([^"]+)"\) != ""$/)
  if (m) return { source: 'header', path: m[1], mode: MATCH_EXISTS, value: '' }

  m = expr.match(/^param\("([^"]+)"\) != nil$/)
  if (m) return { source: 'param', path: m[1], mode: MATCH_EXISTS, value: '' }

  m = expr.match(/^has\(header\("([^"]+)"\), ((?:"(?:[^"\\]|\\.)*"))\)$/)
  if (m) {
    return {
      source: 'header',
      path: m[1],
      mode: MATCH_CONTAINS,
      value: JSON.parse(m[2]) as string,
    }
  }

  m = expr.match(
    /^param\("([^"]+)"\) != nil && has\(param\("([^"]+)"\), ((?:"(?:[^"\\]|\\.)*"))\)$/
  )
  if (m && m[1] === m[2]) {
    return {
      source: 'param',
      path: m[1],
      mode: MATCH_CONTAINS,
      value: JSON.parse(m[3]) as string,
    }
  }

  m = expr.match(
    /^param\("([^"]+)"\) != nil && param\("([^"]+)"\) (>|>=|<|<=) ([\d.eE+-]+)$/
  )
  if (m && m[1] === m[2]) {
    const opMap: Record<string, string> = {
      '>': MATCH_GT,
      '>=': MATCH_GTE,
      '<': MATCH_LT,
      '<=': MATCH_LTE,
    }
    return { source: 'param', path: m[1], mode: opMap[m[3]], value: m[4] }
  }

  m = expr.match(/^(param|header)\("([^"]+)"\) == (.+)$/)
  if (m) {
    const parsedValue = parseExprLiteral(m[3])
    if (parsedValue === null) return null
    return {
      source: m[1] as 'param' | 'header',
      path: m[2],
      mode: MATCH_EQ,
      value: String(parsedValue),
    }
  }

  return null
}

function tryParseRuleGroupFactor(part: string): RequestRuleGroup | null {
  const m = part.match(/^\((.+) \? ([\d.eE+-]+) : 1\)$/s)
  if (!m) return null

  const conditionStr = m[1]
  const multiplier = m[2]

  const andParts = splitTopLevelAnd(conditionStr)
  const conditions: RequestCondition[] = []
  for (const ap of andParts) {
    const cond = tryParseRequestCondition(ap.trim())
    if (!cond) return null
    conditions.push(cond)
  }
  if (conditions.length === 0) return null
  return { conditions, multiplier }
}

export function tryParseRequestRuleExpr(
  expr: string
): RequestRuleGroup[] | null {
  const trimmed = (expr || '').trim()
  if (!trimmed) return []

  const parts = splitTopLevelMultiply(trimmed)
  const groups: RequestRuleGroup[] = []
  for (const part of parts) {
    const group = tryParseRuleGroupFactor(part)
    if (!group) return null
    groups.push(group)
  }
  return groups
}

// ---------------------------------------------------------------------------
// Combine / split billing expr and request rules
// ---------------------------------------------------------------------------

export function splitBillingExprAndRequestRules(expr: string): {
  billingExpr: string
  requestRuleExpr: string
} {
  const trimmed = (expr || '').trim()
  if (!trimmed) return { billingExpr: '', requestRuleExpr: '' }

  const parts = splitTopLevelMultiply(trimmed)
  if (parts.length <= 1) return { billingExpr: trimmed, requestRuleExpr: '' }

  const ruleParts: string[] = []
  const baseParts: string[] = []

  parts.forEach((part) => {
    const parsed = tryParseRequestRuleExpr(part)
    if (parsed && parsed.length > 0) {
      ruleParts.push(part)
    } else {
      baseParts.push(part)
    }
  })

  if (ruleParts.length === 0 || baseParts.length !== 1) {
    return { billingExpr: trimmed, requestRuleExpr: '' }
  }

  return {
    billingExpr: unwrapOuterParens(baseParts[0]),
    requestRuleExpr: ruleParts.join(' * '),
  }
}

export function combineBillingExpr(
  baseExpr: string,
  requestRuleExpr: string
): string {
  const base = (baseExpr || '').trim()
  const rules = (requestRuleExpr || '').trim()
  if (!base) return ''
  if (!rules) return base
  return `(${base}) * ${rules}`
}

// ---------------------------------------------------------------------------
// Editor: empty constructors
// ---------------------------------------------------------------------------

export function createEmptyCondition(): ParamHeaderCondition {
  return { source: 'param', path: '', mode: MATCH_EQ, value: '' }
}

export function createEmptyTimeCondition(): TimeCondition {
  return {
    source: 'time',
    timeFunc: 'hour',
    timezone: 'Asia/Shanghai',
    mode: MATCH_GTE,
    value: '',
    rangeStart: '',
    rangeEnd: '',
  }
}

export function createEmptyRuleGroup(): RequestRuleGroup {
  return { conditions: [createEmptyCondition()], multiplier: '' }
}

export function createEmptyTimeRuleGroup(): RequestRuleGroup {
  return { conditions: [createEmptyTimeCondition()], multiplier: '' }
}

// ---------------------------------------------------------------------------
// Editor: match option helpers
// ---------------------------------------------------------------------------

export type MatchOption = { value: string; labelKey: string }

export function getRequestRuleMatchOptions(source: string): MatchOption[] {
  if (source === SOURCE_TIME) {
    return [
      { value: MATCH_EQ, labelKey: 'Equals' },
      { value: MATCH_GTE, labelKey: 'Greater than or equal' },
      { value: MATCH_LT, labelKey: 'Less than' },
      { value: MATCH_RANGE, labelKey: 'Overnight range' },
    ]
  }
  const base: MatchOption[] = [
    { value: MATCH_EQ, labelKey: 'Equals' },
    { value: MATCH_CONTAINS, labelKey: 'Contains' },
    { value: MATCH_EXISTS, labelKey: 'Exists' },
  ]
  if (source === SOURCE_HEADER) return base
  return [
    ...base,
    { value: MATCH_GT, labelKey: 'Greater than' },
    { value: MATCH_GTE, labelKey: 'Greater than or equal' },
    { value: MATCH_LT, labelKey: 'Less than' },
    { value: MATCH_LTE, labelKey: 'Less than or equal' },
  ]
}

// ---------------------------------------------------------------------------
// Editor: normalize a single condition
// ---------------------------------------------------------------------------

function isTimeFunc(value: unknown): value is TimeFunc {
  return typeof value === 'string' && TIME_FUNCS.includes(value as TimeFunc)
}

export function normalizeCondition(
  cond: Partial<RequestCondition> | null | undefined
): RequestCondition {
  let source: RequestCondition['source'] = 'param'
  if (cond?.source === 'time') {
    source = 'time'
  } else if (cond?.source === 'header') {
    source = 'header'
  }

  if (source === 'time') {
    const timeCond = cond as Partial<TimeCondition> | null | undefined
    const timeFunc: TimeFunc = isTimeFunc(timeCond?.timeFunc)
      ? timeCond.timeFunc
      : 'hour'
    const options = getRequestRuleMatchOptions(SOURCE_TIME)
    const mode = options.some((item) => item.value === timeCond?.mode)
      ? (timeCond?.mode as string)
      : MATCH_GTE
    return {
      source: 'time',
      timeFunc,
      timezone: timeCond?.timezone || 'Asia/Shanghai',
      mode,
      value: timeCond?.value == null ? '' : String(timeCond.value),
      rangeStart:
        timeCond?.rangeStart == null ? '' : String(timeCond.rangeStart),
      rangeEnd: timeCond?.rangeEnd == null ? '' : String(timeCond.rangeEnd),
    }
  }

  const phCond = cond as Partial<ParamHeaderCondition> | null | undefined
  const options = getRequestRuleMatchOptions(source)
  const mode = options.some((item) => item.value === phCond?.mode)
    ? (phCond?.mode as string)
    : MATCH_EQ
  return {
    source,
    path: phCond?.path || '',
    mode,
    value: phCond?.value == null ? '' : String(phCond.value),
  }
}

// ---------------------------------------------------------------------------
// Editor: build expression strings
// ---------------------------------------------------------------------------

function buildExprLiteral(mode: string, value: string): string {
  const text = String(value || '').trim()
  if (mode === MATCH_CONTAINS) return JSON.stringify(text)
  if (text === 'true' || text === 'false') return text
  if (NUMERIC_LITERAL_REGEX.test(text)) return text
  return JSON.stringify(text)
}

function buildTimeConditionExpr(cond: TimeCondition): string {
  const normalized = normalizeCondition(cond) as TimeCondition
  const { timeFunc, timezone, mode } = normalized
  const tz = JSON.stringify(timezone)
  const fn = `${timeFunc}(${tz})`

  if (mode === MATCH_RANGE) {
    const s = normalized.rangeStart.trim()
    const e = normalized.rangeEnd.trim()
    if (!NUMERIC_LITERAL_REGEX.test(s) || !NUMERIC_LITERAL_REGEX.test(e)) {
      return ''
    }
    return `${fn} >= ${s} || ${fn} < ${e}`
  }
  const v = normalized.value.trim()
  if (!NUMERIC_LITERAL_REGEX.test(v)) return ''
  const opMap: Record<string, string> = {
    [MATCH_EQ]: '==',
    [MATCH_GTE]: '>=',
    [MATCH_LT]: '<',
  }
  return `${fn} ${opMap[mode] || '=='} ${v}`
}

function buildRequestConditionExpr(cond: RequestCondition): string {
  if (cond.source === 'time') return buildTimeConditionExpr(cond)
  const normalized = normalizeCondition(cond) as ParamHeaderCondition
  const path = normalized.path.trim()
  if (!path) return ''

  const sourceExpr =
    normalized.source === 'header'
      ? `header(${JSON.stringify(path)})`
      : `param(${JSON.stringify(path)})`

  switch (normalized.mode) {
    case MATCH_EXISTS:
      return normalized.source === 'header'
        ? `${sourceExpr} != ""`
        : `${sourceExpr} != nil`
    case MATCH_CONTAINS:
      return normalized.source === 'header'
        ? `has(${sourceExpr}, ${buildExprLiteral(normalized.mode, normalized.value)})`
        : `${sourceExpr} != nil && has(${sourceExpr}, ${buildExprLiteral(normalized.mode, normalized.value)})`
    case MATCH_GT:
    case MATCH_GTE:
    case MATCH_LT:
    case MATCH_LTE: {
      const opMap: Record<string, string> = {
        [MATCH_GT]: '>',
        [MATCH_GTE]: '>=',
        [MATCH_LT]: '<',
        [MATCH_LTE]: '<=',
      }
      const numText = String(normalized.value).trim()
      if (!NUMERIC_LITERAL_REGEX.test(numText)) return ''
      return `${sourceExpr} != nil && ${sourceExpr} ${opMap[normalized.mode]} ${numText}`
    }
    case MATCH_EQ:
    default:
      return `${sourceExpr} == ${buildExprLiteral(normalized.mode, normalized.value)}`
  }
}

function buildRuleGroupFactor(group: RequestRuleGroup): string {
  const multiplier = (group.multiplier || '').trim()
  if (!NUMERIC_LITERAL_REGEX.test(multiplier)) return ''
  const condExprs = (group.conditions || [])
    .map(buildRequestConditionExpr)
    .filter(Boolean)
  if (condExprs.length === 0) return ''

  const combined =
    condExprs.length === 1
      ? condExprs[0]
      : condExprs.map((e) => (e.includes(' || ') ? `(${e})` : e)).join(' && ')
  return `(${combined} ? ${multiplier} : 1)`
}

export function buildRequestRuleExpr(groups: RequestRuleGroup[]): string {
  return (groups || []).map(buildRuleGroupFactor).filter(Boolean).join(' * ')
}
