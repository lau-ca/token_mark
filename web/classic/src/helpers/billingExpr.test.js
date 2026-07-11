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

import test from 'node:test';
import assert from 'node:assert/strict';
import { parseTierCalls, parseTiersFromExpr } from './billingExpr.js';

test('parses request and duration prices from nested tier calls', () => {
  const tiers = parseTiersFromExpr(`
    param("resolution") == "480p"
      ? tier("480p", per_request(5.5))
      : param("resolution") == "1080p"
        ? tier("1080p", per_request(0.9) * param("duration"))
        : tier("invalid", -1)
  `);

  assert.equal(tiers.length, 2);
  assert.deepEqual(
    tiers.map(({ label, billingUnit, requestPrice, secondPrice }) => ({
      label,
      billingUnit,
      requestPrice,
      secondPrice,
    })),
    [
      {
        label: '480p',
        billingUnit: 'request',
        requestPrice: 5.5,
        secondPrice: undefined,
      },
      {
        label: '1080p',
        billingUnit: 'second',
        requestPrice: undefined,
        secondPrice: 0.9,
      },
    ],
  );
});

test('keeps historical token and constant expressions compatible', () => {
  const tokenTier = parseTiersFromExpr(
    'len <= 200000 ? tier("standard", p * 3 + c * 15 + cr * 0.3) : tier("long", p * 6 + c * 22.5)',
  );
  assert.equal(tokenTier.length, 2);
  assert.equal(tokenTier[0].billingUnit, 'token');
  assert.equal(tokenTier[0].inputPrice, 3);
  assert.equal(tokenTier[0].outputPrice, 15);
  assert.equal(tokenTier[0].cacheReadPrice, 0.3);
  assert.deepEqual(tokenTier[0].conditions, [
    { var: 'len', op: '<=', value: 200000 },
  ]);

  const constantTier = parseTiersFromExpr('tier("base", 2500000)');
  assert.equal(constantTier[0].requestPrice, 2.5);

  const defaultTier = parseTiersFromExpr('tier("default", p)');
  assert.equal(defaultTier[0].inputPrice, 1);

  const legacyTier = parseTiersFromExpr('tier("legacy", (p) * 2 + c * 8)');
  assert.equal(legacyTier[0].inputPrice, 2);
  assert.equal(legacyTier[0].outputPrice, 8);
});

test('does not mistake numeric token multiplication for request pricing', () => {
  assert.deepEqual(parseTiersFromExpr('tier("base", 2500000 * p)'), []);
  assert.deepEqual(
    parseTiersFromExpr(
      'tier("base", per_request(0.9) * max(param("duration"), 1))',
    ),
    [],
  );
});

test('falls back when tier calls are partial branches or repeat token variables', () => {
  const expressions = [
    'tier("base", per_request(2)) + per_request(3)',
    'tier("base", p * 2) * 3',
    'tier("base", p * 2 + p * 3)',
  ];

  for (const expression of expressions) {
    assert.deepEqual(parseTiersFromExpr(expression), []);
  }
});

test('balanced scanner preserves nested function bodies and ignores request rules', () => {
  const calls = parseTierCalls(
    'v1:tier("base", per_request(2) * param("duration"))|||when(header("x") has "y") * 2',
  );

  assert.equal(calls.length, 1);
  assert.equal(calls[0].label, 'base');
  assert.equal(calls[0].body, 'per_request(2) * param("duration")');
});
