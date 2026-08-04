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

import { useTranslation } from "react-i18next";

import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";

import { getPlaygroundParameterValidationError } from "../../lib";
import type { PlaygroundParameterOption } from "../../types";

export type PlaygroundParameterValues = Record<
  string,
  string | number | boolean
>;

type Props = {
  className?: string;
  parameters: PlaygroundParameterOption[];
  showErrors?: boolean;
  values: PlaygroundParameterValues;
  onChange: (values: PlaygroundParameterValues) => void;
};

export function PlaygroundParameterFields(props: Props) {
  const { t } = useTranslation();

  const updateValue = (key: string, value: string | number | boolean) => {
    props.onChange({ ...props.values, [key]: value });
  };

  if (props.parameters.length === 0) return null;

  return (
    <FieldGroup
      className={cn(
        "grid gap-3 sm:grid-cols-2 lg:grid-cols-3",
        props.className,
      )}
    >
      {props.parameters.map((parameter) => {
        const controlId = `playground-media-${parameter.key}`;
        const value = props.values[parameter.key];
        const validationError = getPlaygroundParameterValidationError(
          parameter,
          value,
        );
        const isInvalid = validationError !== null;
        let control;
        if (parameter.type === "enum") {
          control = (
            <Select
              value={String(value ?? "")}
              onValueChange={(value) => {
                if (value !== null) updateValue(parameter.key, value);
              }}
            >
              <SelectTrigger
                aria-invalid={props.showErrors && isInvalid}
                id={controlId}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {(parameter.options ?? []).map((option) => (
                    <SelectItem key={String(option)} value={String(option)}>
                      {String(option)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          );
        } else if (parameter.type === "boolean") {
          control = (
            <Switch
              checked={Boolean(value)}
              id={controlId}
              onCheckedChange={(checked) => updateValue(parameter.key, checked)}
            />
          );
        } else {
          control = (
            <Input
              aria-invalid={props.showErrors && isInvalid}
              id={controlId}
              type={parameter.type === "number" ? "number" : "text"}
              min={parameter.min}
              max={parameter.max}
              value={String(value ?? "")}
              onChange={(event) => {
                const value = event.target.value;
                updateValue(
                  parameter.key,
                  parameter.type === "number" && value !== ""
                    ? Number(value)
                    : value,
                );
              }}
            />
          );
        }

        const label = t(parameter.label || parameter.key);
        const rangeDescription =
          parameter.min !== undefined || parameter.max !== undefined
            ? t("Range: {{min}} to {{max}}", {
                min: parameter.min ?? "−∞",
                max: parameter.max ?? "+∞",
              })
            : null;

        if (parameter.type === "boolean") {
          return (
            <Field
              data-invalid={props.showErrors && isInvalid}
              key={parameter.key}
              orientation="horizontal"
            >
              <FieldContent>
                <FieldLabel htmlFor={controlId}>
                  {label}
                  {parameter.required && (
                    <span className="text-muted-foreground text-xs font-normal">
                      {t("Required")}
                    </span>
                  )}
                </FieldLabel>
                {rangeDescription && (
                  <FieldDescription>{rangeDescription}</FieldDescription>
                )}
              </FieldContent>
              {control}
              {props.showErrors && validationError === "required" && (
                <FieldError>{t("This parameter is required.")}</FieldError>
              )}
            </Field>
          );
        }

        return (
          <Field
            className="gap-1.5"
            data-invalid={props.showErrors && isInvalid}
            key={parameter.key}
          >
            <FieldLabel htmlFor={controlId}>
              {label}
              {parameter.required && (
                <span className="text-muted-foreground text-xs font-normal">
                  {t("Required")}
                </span>
              )}
            </FieldLabel>
            {control}
            {rangeDescription && (
              <FieldDescription className="text-xs">
                {rangeDescription}
              </FieldDescription>
            )}
            {props.showErrors && validationError === "required" && (
              <FieldError className="text-xs">
                {t("This parameter is required.")}
              </FieldError>
            )}
            {props.showErrors && validationError === "range" && (
              <FieldError className="text-xs">
                {t("Enter a value between {{min}} and {{max}}.", {
                  min: parameter.min ?? "−∞",
                  max: parameter.max ?? "+∞",
                })}
              </FieldError>
            )}
          </Field>
        );
      })}
    </FieldGroup>
  );
}
