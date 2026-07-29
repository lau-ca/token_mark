import assert from 'node:assert/strict'
import { test } from 'node:test'

import { parseRequestErrorDetails } from './request-error-utils'

test('uses the server error message instead of the generic HTTP status', () => {
  const result = parseRequestErrorDetails({
    message: 'Request failed with status code 400',
    response: {
      data: {
        error: {
          code: 'invalid_video_request',
          message: 'A reference image is required for this model',
        },
      },
    },
  })

  assert.deepEqual(result, {
    errorCode: 'invalid_video_request',
    errorMessage: 'A reference image is required for this model',
  })
})
