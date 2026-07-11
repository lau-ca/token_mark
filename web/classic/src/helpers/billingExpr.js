/*
Copyright (C) 2025 QuantumNous

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

import {
  BILLING_PRICING_VARS,
  BILLING_VAR_KEY_TO_FIELD,
} from '../constants/billing.constants.js';

const NUMBER_SOURCE = '[+-]?(?:\\d+(?:\\.\\d*)?|\\.\\d+)(?:[eE][+-]?\\d+)?';
const NUMBER_REGEX = new RegExp(`^${NUMBER_SOURCE}$`);
const BILLING_VAR_NAME_SET = new Set(
  BILLING_PRICING_VARS.map((variable) => variable.key),
);
const TOKEN_CONDITION_REGEX = new RegExp(
  `^(p|c|len)\\s*(<=|>=|<|>)\\s*(${NUMBER_SOURCE})$`,
);
const TOKEN_CONDITION_GROUP_SOURCE = `(?:(?:p|c|len)\\s*(?:<=|>=|<|>)\\s*${NUMBER_SOURCE})(?:\\s*&&\\s*(?:p|c|len)\\s*(?:<=|>=|<|>)\\s*${NUMBER_SOURCE})*`;
const PER_REQUEST_REGEX = new RegExp(
  `^per_request\\s*\\(\\s*(${NUMBER_SOURCE})\\s*\\)$`,
);
const DURATION_PARAM_SOURCE =
  'param\\s*\\(\\s*(?:"duration"|\'duration\')\\s*\\)';
const PER_REQUEST_DURATION_REGEX = new RegExp(
  `^per_request\\s*\\(\\s*(${NUMBER_SOURCE})\\s*\\)\\s*\\*\\s*${DURATION_PARAM_SOURCE}$`,
);
const DURATION_PER_REQUEST_REGEX = new RegExp(
  `^${DURATION_PARAM_SOURCE}\\s*\\*\\s*per_request\\s*\\(\\s*(${NUMBER_SOURCE})\\s*\\)$`,
);

const isIdentifierChar = (char) => /[A-Za-z0-9_]/.test(char || '');

function findMatchingParenthesis(source, openIndex) {
  let depth = 0;
  let quote = '';
  let escaped = false;

  for (let index = openIndex; index < source.length; index++) {
    const char = source[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === '\\') {
        escaped = true;
      } else if (char === quote) {
        quote = '';
      }
      continue;
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char;
    } else if (char === '(') {
      depth++;
    } else if (char === ')') {
      depth--;
      if (depth === 0) return index;
      if (depth < 0) return -1;
    }
  }

  return -1;
}

function splitTopLevelArguments(source) {
  const argumentsList = [];
  let start = 0;
  let parentheses = 0;
  let brackets = 0;
  let braces = 0;
  let quote = '';
  let escaped = false;

  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === '\\') {
        escaped = true;
      } else if (char === quote) {
        quote = '';
      }
      continue;
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char;
      continue;
    }

    if (char === '(') parentheses++;
    else if (char === ')') parentheses--;
    else if (char === '[') brackets++;
    else if (char === ']') brackets--;
    else if (char === '{') braces++;
    else if (char === '}') braces--;
    else if (
      char === ',' &&
      parentheses === 0 &&
      brackets === 0 &&
      braces === 0
    ) {
      argumentsList.push(source.slice(start, index).trim());
      start = index + 1;
    }
  }

  argumentsList.push(source.slice(start).trim());
  return argumentsList;
}

function parseTierLabel(source) {
  const value = source.trim();
  if (value.length < 2 || value[0] !== value[value.length - 1]) return null;

  const quote = value[0];
  if (quote === '"') {
    try {
      return JSON.parse(value);
    } catch {
      return null;
    }
  }
  if (quote !== "'") return null;

  return value.slice(1, -1).replace(/\\'/g, "'").replace(/\\\\/g, '\\');
}

function parseTokenConditions(source, tierStart) {
  const prefix = source.slice(0, tierStart);
  const match = prefix.match(
    new RegExp(`(${TOKEN_CONDITION_GROUP_SOURCE})\\s*\\?\\s*$`),
  );
  if (!match) return [];

  return match[1].split(/\s*&&\s*/).map((condition) => {
    const conditionMatch = condition.trim().match(TOKEN_CONDITION_REGEX);
    return {
      var: conditionMatch[1],
      op: conditionMatch[2],
      value: Number(conditionMatch[3]),
    };
  });
}

function isDirectTierBranch(source, start, end) {
  let previousIndex = start - 1;
  while (previousIndex >= 0 && /\s/.test(source[previousIndex]))
    previousIndex--;

  if (previousIndex >= 0) {
    const previousChar = source[previousIndex];
    if (!['?', ':', '('].includes(previousChar)) return false;
    if (previousChar === '(') {
      let beforeParenthesis = previousIndex - 1;
      while (beforeParenthesis >= 0 && /\s/.test(source[beforeParenthesis])) {
        beforeParenthesis--;
      }
      if (isIdentifierChar(source[beforeParenthesis])) return false;
    }
  }

  let nextIndex = end + 1;
  while (nextIndex < source.length && /\s/.test(source[nextIndex])) nextIndex++;
  if (nextIndex >= source.length) return true;
  return [':', ')'].includes(source[nextIndex]);
}

function stripOuterParentheses(source) {
  let value = source.trim();
  while (value.startsWith('(')) {
    const closeIndex = findMatchingParenthesis(value, 0);
    if (closeIndex !== value.length - 1) break;
    value = value.slice(1, -1).trim();
  }
  return value;
}

function splitTopLevelMultiplication(source) {
  const parts = [];
  let start = 0;
  let depth = 0;

  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    if (char === '(') {
      depth++;
      continue;
    }
    if (char === ')') {
      depth--;
      if (depth < 0) return null;
      continue;
    }
    if (depth === 0 && char === '*') {
      parts.push(source.slice(start, index).trim());
      start = index + 1;
    }
  }

  if (depth !== 0) return null;
  parts.push(source.slice(start).trim());
  return parts.every(Boolean) ? parts : null;
}

function splitTopLevelAddition(source) {
  const parts = [];
  let start = 0;
  let depth = 0;

  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    if (char === '(') {
      depth++;
      continue;
    }
    if (char === ')') {
      depth--;
      if (depth < 0) return null;
      continue;
    }
    if (depth !== 0 || char !== '+') continue;

    let previousIndex = index - 1;
    while (previousIndex >= 0 && /\s/.test(source[previousIndex])) {
      previousIndex--;
    }
    const previousChar = source[previousIndex];
    if (
      previousIndex < 0 ||
      ['*', '/', '+', '-', '('].includes(previousChar) ||
      previousChar === 'e' ||
      previousChar === 'E'
    ) {
      continue;
    }

    parts.push(source.slice(start, index).trim());
    start = index + 1;
  }

  if (depth !== 0) return null;
  parts.push(source.slice(start).trim());
  return parts.every(Boolean) ? parts : null;
}

function parseTokenTierBody(body) {
  const terms = splitTopLevelAddition(body);
  if (!terms) return null;

  const coefficients = {};
  for (const term of terms) {
    const factors = splitTopLevelMultiplication(stripOuterParentheses(term));
    if (!factors || factors.length < 1 || factors.length > 2) return null;

    const variableName = stripOuterParentheses(factors[0]);
    if (!BILLING_VAR_NAME_SET.has(variableName)) return null;
    if (variableName in coefficients) return null;

    let coefficient = 1;
    if (factors.length === 2) {
      const coefficientSource = stripOuterParentheses(factors[1]);
      if (!NUMBER_REGEX.test(coefficientSource)) return null;
      coefficient = Number(coefficientSource);
      if (!Number.isFinite(coefficient)) return null;
    }
    coefficients[variableName] = coefficient;
  }

  const tier = { billingUnit: 'token' };
  for (const [variableName, field] of Object.entries(
    BILLING_VAR_KEY_TO_FIELD,
  )) {
    tier[field] = coefficients[variableName] || 0;
  }
  return tier;
}

function parseTierBody(bodySource) {
  const body = stripOuterParentheses(bodySource);

  let match = body.match(PER_REQUEST_REGEX);
  if (match) {
    const requestPrice = Number(match[1]);
    return Number.isFinite(requestPrice)
      ? { billingUnit: 'request', requestPrice }
      : null;
  }

  match = body.match(PER_REQUEST_DURATION_REGEX);
  if (!match) match = body.match(DURATION_PER_REQUEST_REGEX);
  if (match) {
    const secondPrice = Number(match[1]);
    return Number.isFinite(secondPrice)
      ? { billingUnit: 'second', secondPrice }
      : null;
  }

  if (NUMBER_REGEX.test(body)) {
    const requestPrice = Number(body) / 1000000;
    return Number.isFinite(requestPrice)
      ? { billingUnit: 'request', requestPrice }
      : null;
  }
  return parseTokenTierBody(body);
}

export function stripExprVersion(exprStr) {
  if (!exprStr) return { version: 1, body: '' };
  const value = String(exprStr).trim();
  const match = value.match(/^v(\d+):([\s\S]*)$/);
  if (!match) return { version: 1, body: value };
  return { version: Number(match[1]), body: match[2] };
}

export function getBillingExprBody(exprStr) {
  const { body } = stripExprVersion(exprStr);
  const requestRulesIndex = body.indexOf('|||');
  return (
    requestRulesIndex === -1 ? body : body.slice(0, requestRulesIndex)
  ).trim();
}

function hasFunctionCall(source, functionName) {
  let quote = '';
  let escaped = false;

  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === '\\') {
        escaped = true;
      } else if (char === quote) {
        quote = '';
      }
      continue;
    }
    if (char === '"' || char === "'" || char === '`') {
      quote = char;
      continue;
    }
    if (!source.startsWith(functionName, index)) continue;
    if (
      isIdentifierChar(source[index - 1]) ||
      isIdentifierChar(source[index + functionName.length])
    ) {
      continue;
    }

    let openIndex = index + functionName.length;
    while (openIndex < source.length && /\s/.test(source[openIndex])) {
      openIndex++;
    }
    if (
      source[openIndex] === '(' &&
      findMatchingParenthesis(source, openIndex) !== -1
    ) {
      return true;
    }
  }

  return false;
}

export function isRequestDependentBillingExpr(exprStr) {
  const body = getBillingExprBody(exprStr);
  return ['per_request', 'param', 'header'].some((functionName) =>
    hasFunctionCall(body, functionName),
  );
}

export function parseTierCalls(exprStr) {
  const source = getBillingExprBody(exprStr);
  if (!source) return [];

  const calls = [];
  let quote = '';
  let escaped = false;

  for (let index = 0; index < source.length; index++) {
    const char = source[index];
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === '\\') {
        escaped = true;
      } else if (char === quote) {
        quote = '';
      }
      continue;
    }

    if (char === '"' || char === "'" || char === '`') {
      quote = char;
      continue;
    }
    if (
      !source.startsWith('tier', index) ||
      isIdentifierChar(source[index - 1]) ||
      isIdentifierChar(source[index + 4])
    ) {
      continue;
    }

    let openIndex = index + 4;
    while (openIndex < source.length && /\s/.test(source[openIndex]))
      openIndex++;
    if (source[openIndex] !== '(') continue;

    const closeIndex = findMatchingParenthesis(source, openIndex);
    if (closeIndex === -1) return null;
    const argumentsList = splitTopLevelArguments(
      source.slice(openIndex + 1, closeIndex),
    );
    if (argumentsList.length !== 2) return null;

    const label = parseTierLabel(argumentsList[0]);
    if (label === null || !argumentsList[1]) return null;
    calls.push({
      label,
      body: argumentsList[1],
      conditions: parseTokenConditions(source, index),
      start: index,
      end: closeIndex,
      isDirectBranch: isDirectTierBranch(source, index, closeIndex),
    });
    index = closeIndex;
  }

  return calls;
}

export function parseTiersFromExpr(exprStr) {
  const calls = parseTierCalls(exprStr);
  if (
    !calls ||
    calls.length === 0 ||
    calls.some((call) => !call.isDirectBranch)
  ) {
    return [];
  }

  const tiers = [];
  for (const call of calls) {
    const parsedBody = parseTierBody(call.body);
    if (!parsedBody) return [];
    if (
      (Number.isFinite(parsedBody.requestPrice) &&
        parsedBody.requestPrice < 0) ||
      (Number.isFinite(parsedBody.secondPrice) && parsedBody.secondPrice < 0)
    ) {
      continue;
    }
    tiers.push({
      ...parsedBody,
      label: call.label,
      conditions: call.conditions,
    });
  }
  return tiers;
}
