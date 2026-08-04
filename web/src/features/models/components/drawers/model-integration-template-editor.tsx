import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import { CodeBlock } from "@/components/ai-elements/code-block";
import { SideDrawerSection } from "@/components/drawer-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import {
  buildPlaygroundMediaCurl,
  buildPlaygroundMediaPayload,
  resolvePlaygroundIntegrationGuide,
} from "@/features/playground/lib";

import type {
  PlaygroundIntegrationDefinition,
  PlaygroundIntegrationInterfaceDefinition,
  PlaygroundIntegrationMethod,
  PlaygroundParameterDefinition,
} from "../../lib/model-capabilities";

type Props = {
  capabilities: string[];
  endpointType: string;
  model: string;
  onChange: (value: PlaygroundIntegrationDefinition | undefined) => void;
  parameters: PlaygroundParameterDefinition[];
  value?: PlaygroundIntegrationDefinition;
};

const METHODS: PlaygroundIntegrationMethod[] = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
];

export function ModelIntegrationTemplateEditor(props: Props) {
  const { t } = useTranslation();
  const integration = props.value;
  const preview = useMemo(() => {
    if (!props.value) return null;
    const parameterValues = Object.fromEntries(
      props.parameters.flatMap((parameter) =>
        parameter.default === undefined
          ? []
          : [[parameter.key, parameter.default]],
      ),
    );
    const isVideo = props.capabilities.some((capability) =>
      capability.startsWith("video."),
    );
    const isImage = props.capabilities.some((capability) =>
      capability.startsWith("image."),
    );
    const prompt = "Describe what you want to generate";
    const requestPayload =
      isImage || isVideo
        ? buildPlaygroundMediaPayload({
            group: "default",
            model: props.model || "example-model",
            parameters: props.parameters,
            prompt,
            values: parameterValues,
          })
        : {
            model: props.model || "example-model",
            group: "default",
            messages: [{ role: "user", content: prompt }],
          };
    const requestCurl =
      isImage || isVideo
        ? buildPlaygroundMediaCurl({
            apiBaseUrl: "https://api.frimodel.com",
            canEditImage: props.capabilities.includes("image.edit"),
            canGenerateVideoFromImage: props.capabilities.includes(
              "video.image_to_video",
            ),
            endpointType: props.endpointType,
            group: "default",
            mode: isVideo ? "video" : "image",
            model: props.model || "example-model",
            parameters: props.parameters,
            prompt,
            values: parameterValues,
          })
        : "curl 'https://api.frimodel.com/v1/chat/completions'";

    return resolvePlaygroundIntegrationGuide({
      apiBaseUrl: "https://api.frimodel.com",
      definition: props.value,
      group: "default",
      model: props.model || "example-model",
      parameters: parameterValues,
      prompt,
      referenceUrl: "https://example.com/reference.png",
      requestCurl,
      requestJson: JSON.stringify(requestPayload, null, 2),
    });
  }, [
    props.capabilities,
    props.endpointType,
    props.model,
    props.parameters,
    props.value,
  ]);

  const updateRoot = (patch: Partial<PlaygroundIntegrationDefinition>) => {
    if (!props.value) return;
    props.onChange({ ...props.value, ...patch });
  };

  const updateInterface = (
    index: number,
    patch: Partial<PlaygroundIntegrationInterfaceDefinition>,
  ) => {
    if (!props.value) return;
    updateRoot({
      interfaces: props.value.interfaces.map((item, itemIndex) =>
        itemIndex === index ? { ...item, ...patch } : item,
      ),
    });
  };

  const moveInterface = (index: number, offset: number) => {
    if (!props.value) return;
    const nextIndex = index + offset;
    if (nextIndex < 0 || nextIndex >= props.value.interfaces.length) return;
    const interfaces = [...props.value.interfaces];
    const [item] = interfaces.splice(index, 1);
    interfaces.splice(nextIndex, 0, item);
    updateRoot({ interfaces });
  };

  return (
    <SideDrawerSection>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold">{t("API integration")}</h3>
          <p className="text-muted-foreground mt-1 text-xs leading-5">
            {t(
              "Define the interfaces and explanations developers see in the playground.",
            )}
          </p>
        </div>
        <div className="flex gap-2">
          {!integration && (
            <Button
              onClick={() => props.onChange({ interfaces: [] })}
              size="sm"
              type="button"
              variant="outline"
            >
              {t("Configure")}
            </Button>
          )}
          {integration && (
            <Button
              onClick={() => props.onChange(undefined)}
              size="sm"
              type="button"
              variant="ghost"
            >
              {t("Delete")}
            </Button>
          )}
        </div>
      </div>

      {!integration ? (
        <div className="bg-muted/30 text-muted-foreground rounded-md border border-dashed p-4 text-xs leading-5">
          {t("Not configured")}
        </div>
      ) : (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label>{t("Overview")}</Label>
            <Textarea
              className="min-h-20"
              onChange={(event) => updateRoot({ overview: event.target.value })}
              value={integration.overview ?? ""}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{t("Documentation URL")}</Label>
            <Input
              onChange={(event) =>
                updateRoot({ documentation_url: event.target.value })
              }
              placeholder="https://ai-doc.apifox.cn/..."
              value={integration.documentation_url ?? ""}
            />
          </div>

          <div className="flex items-center justify-between gap-3">
            <h4 className="text-sm font-medium">{t("Interfaces")}</h4>
            <Button
              onClick={() =>
                updateRoot({
                  interfaces: [
                    ...integration.interfaces,
                    {
                      key: `interface_${integration.interfaces.length + 1}`,
                      title: "New interface",
                      method: "POST",
                      path: "/",
                      curl_template: "curl '{{base_url}}/'",
                    },
                  ],
                })
              }
              size="sm"
              type="button"
              variant="outline"
            >
              <Plus className="size-4" />
              {t("Add interface")}
            </Button>
          </div>

          <div className="space-y-3">
            {integration.interfaces.map((item, index) => (
              <div className="space-y-3 rounded-lg border p-3" key={item.key}>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs font-medium">
                    {t("Interface {{number}}", { number: index + 1 })}
                  </span>
                  <div className="flex gap-1">
                    <Button
                      aria-label={t("Move up")}
                      disabled={index === 0}
                      onClick={() => moveInterface(index, -1)}
                      size="icon-sm"
                      type="button"
                      variant="ghost"
                    >
                      <ArrowUp className="size-4" />
                    </Button>
                    <Button
                      aria-label={t("Move down")}
                      disabled={index === integration.interfaces.length - 1}
                      onClick={() => moveInterface(index, 1)}
                      size="icon-sm"
                      type="button"
                      variant="ghost"
                    >
                      <ArrowDown className="size-4" />
                    </Button>
                    <Button
                      aria-label={t("Delete interface")}
                      onClick={() =>
                        updateRoot({
                          interfaces: integration.interfaces.filter(
                            (_, itemIndex) => itemIndex !== index,
                          ),
                        })
                      }
                      size="icon-sm"
                      type="button"
                      variant="ghost"
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="space-y-1.5">
                    <Label>{t("Interface key")}</Label>
                    <Input
                      onChange={(event) =>
                        updateInterface(index, { key: event.target.value })
                      }
                      value={item.key}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label>{t("Title")}</Label>
                    <Input
                      onChange={(event) =>
                        updateInterface(index, { title: event.target.value })
                      }
                      value={item.title}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label>{t("Method")}</Label>
                    <Select
                      onValueChange={(value) =>
                        value !== null &&
                        updateInterface(index, {
                          method: value as PlaygroundIntegrationMethod,
                        })
                      }
                      value={item.method}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {METHODS.map((method) => (
                            <SelectItem key={method} value={method}>
                              {method}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-1.5">
                    <Label>{t("Path")}</Label>
                    <Input
                      onChange={(event) =>
                        updateInterface(index, { path: event.target.value })
                      }
                      value={item.path}
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label>{t("Description")}</Label>
                  <Textarea
                    onChange={(event) =>
                      updateInterface(index, {
                        description: event.target.value,
                      })
                    }
                    value={item.description ?? ""}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>{t("Request description")}</Label>
                  <Textarea
                    onChange={(event) =>
                      updateInterface(index, {
                        request_description: event.target.value,
                      })
                    }
                    value={item.request_description ?? ""}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>{t("curl template")}</Label>
                  <Textarea
                    className="min-h-32 font-mono text-xs"
                    onChange={(event) =>
                      updateInterface(index, {
                        curl_template: event.target.value,
                      })
                    }
                    value={item.curl_template}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>{t("Response example")}</Label>
                  <Textarea
                    className="min-h-28 font-mono text-xs"
                    onChange={(event) =>
                      updateInterface(index, {
                        response_example: event.target.value,
                      })
                    }
                    value={item.response_example ?? ""}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label>{t("Notes")}</Label>
                  <Textarea
                    onChange={(event) =>
                      updateInterface(index, {
                        notes: event.target.value
                          .split("\n")
                          .map((note) => note.trim())
                          .filter(Boolean),
                      })
                    }
                    value={(item.notes ?? []).join("\n")}
                  />
                </div>
              </div>
            ))}
          </div>

          <div className="space-y-1.5">
            <Label>{t("Result handling")}</Label>
            <Textarea
              onChange={(event) =>
                updateRoot({ result_note: event.target.value })
              }
              value={integration.result_note ?? ""}
            />
          </div>
          <div className="space-y-1.5">
            <Label>{t("Complete workflow")}</Label>
            <Textarea
              className="min-h-40 font-mono text-xs"
              onChange={(event) =>
                updateRoot({ complete_example: event.target.value })
              }
              value={integration.complete_example ?? ""}
            />
          </div>

          {preview && (
            <div className="space-y-2">
              <Label>{t("Preview")}</Label>
              <div className="space-y-3 rounded-lg border p-3">
                {preview.interfaces.map((item) => (
                  <div className="space-y-2" key={item.key}>
                    <div className="text-xs font-medium">
                      {item.method} {item.path} · {item.title}
                    </div>
                    <CodeBlock code={item.curl} language="bash" />
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </SideDrawerSection>
  );
}
