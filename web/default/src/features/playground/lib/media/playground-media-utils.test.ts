import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getImageResult } from './playground-media-utils'

describe('playground image responses', () => {
  test('corrects a mislabeled markdown data URL from its base64 signature', () => {
    const result = getImageResult({
      choices: [
        {
          message: {
            content: '![generated](data:image/png;base64,/9j/example)',
          },
        },
      ],
    })

    assert.equal(result.media[0]?.url, 'data:image/jpeg;base64,/9j/example')
    assert.equal(result.media[0]?.isTransient, true)
  })
})
