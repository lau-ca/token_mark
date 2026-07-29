import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { Message } from '../../types'
import { sanitizeMessagesOnLoad } from './message-streaming-utils'

function createPendingVideoMessage(media?: Message['media']): Message {
  return {
    key: 'assistant-video',
    from: 'assistant',
    mode: 'video',
    status: 'loading',
    versions: [{ id: 'version-1', content: '' }],
    media,
  }
}

describe('playground message recovery', () => {
  test('keeps a submitted video task pending after reload', () => {
    const message = createPendingVideoMessage([
      { type: 'video', taskId: 'task_123', progress: 42 },
    ])

    const result = sanitizeMessagesOnLoad([message])

    assert.equal(result[0], message)
    assert.equal(result[0]?.status, 'loading')
  })

  test('marks an interrupted request without a task ID as an error', () => {
    const result = sanitizeMessagesOnLoad([createPendingVideoMessage()])

    assert.equal(result[0]?.status, 'error')
  })
})
