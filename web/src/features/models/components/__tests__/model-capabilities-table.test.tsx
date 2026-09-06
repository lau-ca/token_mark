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
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { modelsQueryKeys } = await import('../../lib/query-keys')
const { ModelCapabilitiesTable } = await import('../model-capabilities-table')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'No available channel': 'No available channel',
        Configure: 'Configure',
        'Filter by model name...': 'Filter by model name...',
        'image.generate': 'Image generation',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('model capability catalog', () => {
  after(() => {
    domWindow.close()
  })

  test('keeps saved capabilities configurable when no channel is available', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    queryClient.setQueryData(modelsQueryKeys.capabilities(), {
      success: true,
      data: [
        {
          model_name: 'saved-image-model',
          supported_endpoint_types: ['image-generation'],
          available: false,
          config:
            '{"endpoints":{"image-generation":{"capabilities":["image.generate"]}}}',
        },
      ],
    })

    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={queryClient}>
          <I18nextProvider i18n={i18n}>
            <ModelCapabilitiesTable />
          </I18nextProvider>
        </QueryClientProvider>
      )
    })

    assert.match(container.textContent ?? '', /saved-image-model/)
    assert.match(container.textContent ?? '', /Image generation/)
    assert.match(container.textContent ?? '', /No available channel/)
    assert.ok(
      [...container.querySelectorAll('button')].some(
        (button) => button.textContent?.trim() === 'Configure'
      )
    )

    await act(async () => root.unmount())
    container.remove()
    queryClient.clear()
  })
})
