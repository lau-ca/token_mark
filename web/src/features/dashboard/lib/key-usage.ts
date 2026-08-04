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
import type {
  AggregatedKeyUsage,
  KeyQuotaDataItem,
  QuotaDataItem,
} from "../types";

type DeletedKeyLabel = (tokenId: number) => string;

function keyName(row: KeyQuotaDataItem, deletedKeyLabel: DeletedKeyLabel) {
  if (row.deleted) return deletedKeyLabel(row.token_id);
  return row.token_name || `Key ${row.token_id}`;
}

function keyChartLabel(
  row: KeyQuotaDataItem,
  deletedKeyLabel: DeletedKeyLabel,
) {
  const name = keyName(row, deletedKeyLabel);
  return row.masked_key ? `${name} · ${row.masked_key}` : name;
}

export function buildKeyChartData(
  rows: KeyQuotaDataItem[],
  deletedKeyLabel: DeletedKeyLabel,
): QuotaDataItem[] {
  return rows
    .filter((row) => row.created_at > 0)
    .map((row) => ({
      created_at: row.created_at,
      model_name: keyChartLabel(row, deletedKeyLabel),
      count: Number(row.count) || 0,
      token_used: Number(row.token_used) || 0,
      quota: Number(row.quota) || 0,
    }));
}

export function aggregateKeyUsage(
  rows: KeyQuotaDataItem[],
  deletedKeyLabel: DeletedKeyLabel,
): AggregatedKeyUsage[] {
  const byToken = new Map<number, AggregatedKeyUsage>();
  let totalQuota = 0;

  for (const row of rows) {
    const quota = Number(row.quota) || 0;
    totalQuota += quota;
    const current = byToken.get(row.token_id);
    if (current) {
      current.count += Number(row.count) || 0;
      current.token_used += Number(row.token_used) || 0;
      current.quota += quota;
      current.accessed_time = Math.max(
        current.accessed_time,
        Number(row.accessed_time) || 0,
      );
      continue;
    }

    byToken.set(row.token_id, {
      token_id: row.token_id,
      token_name: keyName(row, deletedKeyLabel),
      masked_key: row.masked_key || "",
      token_status: Number(row.token_status) || 0,
      accessed_time: Number(row.accessed_time) || 0,
      deleted: Boolean(row.deleted),
      count: Number(row.count) || 0,
      token_used: Number(row.token_used) || 0,
      quota,
      share: 0,
    });
  }

  return [...byToken.values()]
    .map((item) => ({
      ...item,
      share: totalQuota > 0 ? item.quota / totalQuota : 0,
    }))
    .sort((a, b) => b.quota - a.quota || a.token_id - b.token_id);
}
