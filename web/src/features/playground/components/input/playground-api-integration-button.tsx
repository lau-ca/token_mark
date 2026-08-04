import { SquareTerminalIcon } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { PromptInputButton } from "@/components/ai-elements/prompt-input";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import type { PlaygroundIntegrationGuide } from "../../lib";
import { PlaygroundApiIntegrationDialog } from "./playground-api-integration-dialog";

type Props = {
  apiBaseUrl: string;
  disabled?: boolean;
  guide: PlaygroundIntegrationGuide | null;
  model: string;
};

export function PlaygroundApiIntegrationButton(props: Props) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const label = t("API integration");

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <PromptInputButton
              aria-label={label}
              className="text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium"
              disabled={props.disabled || !props.guide || !props.model}
              onClick={() => setOpen(true)}
              variant="ghost"
            >
              <SquareTerminalIcon size={16} />
            </PromptInputButton>
          }
        />
        <TooltipContent>
          <p>{label}</p>
        </TooltipContent>
      </Tooltip>

      {props.guide && (
        <PlaygroundApiIntegrationDialog
          apiBaseUrl={props.apiBaseUrl}
          guide={props.guide}
          model={props.model}
          onOpenChange={setOpen}
          open={open}
        />
      )}
    </>
  );
}
