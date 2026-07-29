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

import type { Message, PlaygroundMedia } from '../../types'

const DATABASE_NAME = 'new-api-playground-media'
const DATABASE_VERSION = 1
const MEDIA_STORE_NAME = 'media'

type StoredMedia = {
  key: string
  blob: Blob
  filename: string
  createdAt: number
}

export type HydratedPlaygroundMedia = {
  messages: Message[]
  objectUrls: Map<string, string>
}

let databasePromise: Promise<IDBDatabase> | null = null

function openMediaDatabase(): Promise<IDBDatabase> {
  if (databasePromise) return databasePromise

  databasePromise = new Promise((resolve, reject) => {
    if (!globalThis.indexedDB) {
      reject(new Error('IndexedDB is unavailable'))
      return
    }

    const request = indexedDB.open(DATABASE_NAME, DATABASE_VERSION)
    request.addEventListener('upgradeneeded', () => {
      const database = request.result
      if (!database.objectStoreNames.contains(MEDIA_STORE_NAME)) {
        database.createObjectStore(MEDIA_STORE_NAME, { keyPath: 'key' })
      }
    })
    request.addEventListener('success', () => resolve(request.result))
    request.addEventListener('error', () => {
      databasePromise = null
      reject(request.error ?? new Error('Failed to open IndexedDB'))
    })
  })

  return databasePromise
}

function waitForTransaction(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    transaction.addEventListener('complete', () => resolve())
    transaction.addEventListener('abort', () =>
      reject(transaction.error ?? new Error('IndexedDB transaction aborted'))
    )
    transaction.addEventListener('error', () =>
      reject(transaction.error ?? new Error('IndexedDB transaction failed'))
    )
  })
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.addEventListener('success', () => resolve(request.result))
    request.addEventListener('error', () =>
      reject(request.error ?? new Error('IndexedDB request failed'))
    )
  })
}

function getImageExtension(mimeType: string): string {
  if (mimeType === 'image/jpeg') return 'jpg'
  if (mimeType === 'image/webp') return 'webp'
  return 'png'
}

function dataUrlToBlob(dataUrl: string): Blob {
  const separatorIndex = dataUrl.indexOf(',')
  if (
    separatorIndex === -1 ||
    !dataUrl.slice(0, separatorIndex).includes(';base64')
  ) {
    throw new Error('Invalid image data URL')
  }

  const metadata = dataUrl.slice(0, separatorIndex)
  const mimeType = metadata.slice(5, metadata.indexOf(';')) || 'image/png'
  const decoded = atob(dataUrl.slice(separatorIndex + 1))
  const bytes = new Uint8Array(decoded.length)
  for (let index = 0; index < decoded.length; index++) {
    bytes[index] = decoded.charCodeAt(index)
  }
  return new Blob([bytes], { type: mimeType })
}

async function saveMedia(blob: Blob, filename: string): Promise<string> {
  const database = await openMediaDatabase()
  const key = crypto.randomUUID()
  const transaction = database.transaction(MEDIA_STORE_NAME, 'readwrite')
  transaction.objectStore(MEDIA_STORE_NAME).put({
    key,
    blob,
    filename,
    createdAt: Date.now(),
  } satisfies StoredMedia)
  await waitForTransaction(transaction)
  return key
}

async function loadMedia(key: string): Promise<StoredMedia | undefined> {
  const database = await openMediaDatabase()
  const transaction = database.transaction(MEDIA_STORE_NAME, 'readonly')
  return requestResult(
    transaction.objectStore(MEDIA_STORE_NAME).get(key) as IDBRequest<
      StoredMedia | undefined
    >
  )
}

export async function persistGeneratedImages(
  media: PlaygroundMedia[]
): Promise<PlaygroundMedia[]> {
  return Promise.all(
    media.map(async (item, index) => {
      if (
        item.type !== 'image' ||
        item.storageKey ||
        !item.url?.startsWith('data:image/')
      ) {
        return item
      }

      try {
        const blob = dataUrlToBlob(item.url)
        const filename =
          item.filename ??
          `generated-image-${Date.now()}-${index + 1}.${getImageExtension(blob.type)}`
        const storageKey = await saveMedia(blob, filename)
        return { ...item, storageKey, filename, isTransient: true }
      } catch {
        return item
      }
    })
  )
}

export async function hydrateStoredMedia(
  messages: Message[]
): Promise<HydratedPlaygroundMedia> {
  const objectUrls = new Map<string, string>()
  const hydratedMessages = await Promise.all(
    messages.map(async (message) => {
      if (!message.media?.some((item) => item.storageKey && !item.url)) {
        return message
      }

      const media = await Promise.all(
        message.media.map(async (item) => {
          if (!item.storageKey || item.url) return item

          try {
            const stored = await loadMedia(item.storageKey)
            if (!stored) return item
            const url = URL.createObjectURL(stored.blob)
            objectUrls.set(item.storageKey, url)
            return {
              ...item,
              url,
              filename: item.filename ?? stored.filename,
              isTransient: true,
            }
          } catch {
            return item
          }
        })
      )
      return { ...message, media }
    })
  )

  return { messages: hydratedMessages, objectUrls }
}

export function getStoredMediaKeys(messages: Message[]): Set<string> {
  const keys = new Set<string>()
  for (const message of messages) {
    for (const item of message.media ?? []) {
      if (item.storageKey) keys.add(item.storageKey)
    }
  }
  return keys
}

export async function deleteStoredMedia(keys: Iterable<string>): Promise<void> {
  const keyList = [...keys]
  if (keyList.length === 0) return

  try {
    const database = await openMediaDatabase()
    const transaction = database.transaction(MEDIA_STORE_NAME, 'readwrite')
    const store = transaction.objectStore(MEDIA_STORE_NAME)
    for (const key of keyList) store.delete(key)
    await waitForTransaction(transaction)
  } catch {
    // Browser storage cleanup is best-effort and must not break the playground.
  }
}

export async function pruneStoredMedia(activeKeys: Set<string>): Promise<void> {
  try {
    const database = await openMediaDatabase()
    const transaction = database.transaction(MEDIA_STORE_NAME, 'readonly')
    const storedKeys = await requestResult(
      transaction.objectStore(MEDIA_STORE_NAME).getAllKeys()
    )
    await deleteStoredMedia(
      storedKeys.map(String).filter((storedKey) => !activeKeys.has(storedKey))
    )
  } catch {
    // Browser storage cleanup is best-effort and must not break the playground.
  }
}
