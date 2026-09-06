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

export interface ModelCapabilityEndpointDefinition {
  capabilities?: PlaygroundCapability[];
  parameters?: PlaygroundParameterDefinition[];
  integration?: PlaygroundIntegrationDefinition;
  [key: string]: unknown;
}

export interface ModelCapabilityConfigDocument {
  endpoints: Record<string, ModelCapabilityEndpointDefinition>;
  [key: string]: unknown;
}

export function parseModelCapabilityConfig(
  raw?: string,
): ModelCapabilityConfigDocument {
  if (!raw?.trim()) return { endpoints: {} };

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (parsed && typeof parsed === "object") {
      const document = parsed as Partial<ModelCapabilityConfigDocument>;
      return {
        ...document,
        endpoints:
          document.endpoints && typeof document.endpoints === "object"
            ? document.endpoints
            : {},
      } as ModelCapabilityConfigDocument;
    }
  } catch {
    return { endpoints: {} };
  }
  return { endpoints: {} };
}

export function getCapabilityEndpointDefinition(
  config: ModelCapabilityConfigDocument,
  endpointName: string,
): ModelCapabilityEndpointDefinition {
  const current = config.endpoints[endpointName];
  return current ? { ...current } : {};
}

export function getModelCapabilities(raw?: string): PlaygroundCapability[] {
  const capabilities = new Set<PlaygroundCapability>();
  const config = parseModelCapabilityConfig(raw);

  for (const endpoint of Object.values(config.endpoints)) {
    for (const capability of endpoint.capabilities ?? []) {
      capabilities.add(capability);
    }
  }
  return [...capabilities];
}

export function serializeModelCapabilityConfig(
  config: ModelCapabilityConfigDocument,
): string {
  return JSON.stringify(config, null, 2);
}
