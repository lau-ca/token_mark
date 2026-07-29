import type { PlaygroundIntegrationDefinition } from '../../types'

export type PlaygroundIntegrationVariables = {
  api_key: string
  base_url: string
  group: string
  model: string
  parameters_json: string
  prompt: string
  reference_url: string
  request_curl: string
  request_json: string
  task_id: string
}

export type PlaygroundIntegrationGuideInterface = {
  key: string
  title: string
  description?: string
  method: string
  path: string
  requestDescription?: string
  curl: string
  responseExample?: string
  notes: string[]
}

export type PlaygroundIntegrationGuide = {
  overview?: string
  documentationUrl?: string
  interfaces: PlaygroundIntegrationGuideInterface[]
  resultNote?: string
  completeExample?: string
}

type ResolveOptions = {
  apiBaseUrl: string
  definition: PlaygroundIntegrationDefinition
  group: string
  model: string
  parameters: Record<string, string | number | boolean>
  prompt: string
  referenceUrl?: string
  requestCurl: string
  requestJson: string
}

const integrationVariablePattern = /\{\{\s*([a-z_]+)\s*\}\}/g

function shellSingleQuotedContent(value: string): string {
  return value.replaceAll("'", `'"'"'`)
}

export function resolveIntegrationTemplate(
  template: string,
  variables: PlaygroundIntegrationVariables
): string {
  return template.replace(
    integrationVariablePattern,
    (match, name: keyof PlaygroundIntegrationVariables) =>
      Object.hasOwn(variables, name) ? variables[name] : match
  )
}

export function resolvePlaygroundIntegrationGuide(
  options: ResolveOptions
): PlaygroundIntegrationGuide {
  const variables: PlaygroundIntegrationVariables = {
    api_key: '$NEW_API_KEY',
    base_url: options.apiBaseUrl.replace(/\/+$/, ''),
    group: options.group,
    model: options.model,
    parameters_json: shellSingleQuotedContent(
      JSON.stringify(options.parameters, null, 2)
    ),
    prompt: shellSingleQuotedContent(options.prompt.trim()),
    reference_url:
      options.referenceUrl?.trim() || 'https://example.com/reference.png',
    request_curl: options.requestCurl,
    request_json: shellSingleQuotedContent(options.requestJson),
    task_id: '$TASK_ID',
  }

  return {
    overview: options.definition.overview,
    documentationUrl: options.definition.documentation_url,
    interfaces: options.definition.interfaces.map((item) => ({
      key: item.key,
      title: item.title,
      description: item.description,
      method: item.method,
      path: resolveIntegrationTemplate(item.path, variables),
      requestDescription: item.request_description,
      curl: resolveIntegrationTemplate(item.curl_template, variables),
      responseExample: item.response_example
        ? resolveIntegrationTemplate(item.response_example, variables)
        : undefined,
      notes: item.notes ?? [],
    })),
    resultNote: options.definition.result_note,
    completeExample: options.definition.complete_example
      ? resolveIntegrationTemplate(
          options.definition.complete_example,
          variables
        )
      : undefined,
  }
}
