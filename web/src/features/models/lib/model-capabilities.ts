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

export type PlaygroundCapability =
  | "chat"
  | "image.generate"
  | "image.edit"
  | "video.text_to_video"
  | "video.image_to_video";

export type PlaygroundParameterType = "string" | "number" | "boolean" | "enum";

export interface PlaygroundParameterDefinition {
  key: string;
  label?: string;
  type: PlaygroundParameterType;
  request_path?: string;
  required?: boolean;
  default?: string | number | boolean;
  options?: Array<string | number | boolean>;
  min?: number;
  max?: number;
}

export type PlaygroundIntegrationMethod =
  | "GET"
  | "POST"
  | "PUT"
  | "PATCH"
  | "DELETE";

export interface PlaygroundIntegrationInterfaceDefinition {
  key: string;
  title: string;
  description?: string;
  method: PlaygroundIntegrationMethod;
  path: string;
  request_description?: string;
  curl_template: string;
  response_example?: string;
  notes?: string[];
}

export interface PlaygroundIntegrationDefinition {
  overview?: string;
  documentation_url?: string;
  interfaces: PlaygroundIntegrationInterfaceDefinition[];
  result_note?: string;
  complete_example?: string;
}

export interface PlaygroundEndpointDefinition {
  path?: string;
  method?: string;
  playground?: {
    capabilities?: PlaygroundCapability[];
    parameters?: PlaygroundParameterDefinition[];
    integration?: PlaygroundIntegrationDefinition;
  };
  [key: string]: unknown;
}

export type ModelEndpointDefinitions = Record<
  string,
  string | PlaygroundEndpointDefinition
>;

export function parseModelEndpointDefinitions(
  raw?: string,
): ModelEndpointDefinitions {
  if (!raw?.trim()) return {};

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (Array.isArray(parsed)) {
      return Object.fromEntries(
        parsed
          .filter((value): value is string => typeof value === "string")
          .map((endpoint) => [endpoint, {}]),
      );
    }
    if (parsed && typeof parsed === "object") {
      return parsed as ModelEndpointDefinitions;
    }
  } catch {
    return {};
  }
  return {};
}

export function getEndpointDefinition(
  endpoints: ModelEndpointDefinitions,
  endpointName: string,
): PlaygroundEndpointDefinition {
  const current = endpoints[endpointName];
  if (typeof current === "string") {
    return { path: current, method: "POST" };
  }
  return current ? { ...current } : {};
}

export function getModelCapabilities(raw?: string): PlaygroundCapability[] {
  const capabilities = new Set<PlaygroundCapability>();
  const endpoints = parseModelEndpointDefinitions(raw);

  for (const value of Object.values(endpoints)) {
    const definition = typeof value === "string" ? { path: value } : value;
    for (const capability of definition.playground?.capabilities ?? []) {
      capabilities.add(capability);
    }
  }
  return [...capabilities];
}

export function serializeModelEndpointDefinitions(
  endpoints: ModelEndpointDefinitions,
): string {
  return JSON.stringify(endpoints, null, 2);
}
