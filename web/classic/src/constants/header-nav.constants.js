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

export const defaultHeaderNavModules = {
  home: true,
  console: true,
  pricing: {
    enabled: true,
    requireAuth: false,
  },
  gateway: true,
  docs: true,
  about: true,
};

export function normalizeHeaderNavModules(modules = {}) {
  const normalizedModules = {
    ...defaultHeaderNavModules,
    ...modules,
  };

  if (typeof normalizedModules.pricing === 'boolean') {
    normalizedModules.pricing = {
      enabled: normalizedModules.pricing,
      requireAuth: false,
    };
  } else {
    normalizedModules.pricing = {
      ...defaultHeaderNavModules.pricing,
      ...normalizedModules.pricing,
    };
  }

  return normalizedModules;
}
