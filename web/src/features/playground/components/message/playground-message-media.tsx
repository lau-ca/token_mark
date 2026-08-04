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

import { DownloadIcon } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";

import { downloadPlaygroundMedia } from "../../lib/media/playground-media-utils";
import type { PlaygroundMedia } from "../../types";

type Props = {
  items: PlaygroundMedia[];
};

function getSafeMediaUrl(item: PlaygroundMedia): string | undefined {
  const url = item.url?.trim();
  if (!url) return undefined;
  if (url.startsWith("/") || url.startsWith("blob:")) return url;
  if (item.type === "image" && url.startsWith("data:image/")) return url;
  if (url.startsWith("https://") || url.startsWith("http://")) return url;
  return undefined;
}

export function PlaygroundMessageMedia(props: Props) {
  const { t } = useTranslation();
  const [downloadingKey, setDownloadingKey] = useState<string | null>(null);

  const handleDownload = async (
    item: PlaygroundMedia,
    url: string,
    index: number,
  ) => {
    const itemKey = item.storageKey ?? item.taskId ?? `${item.type}-${index}`;
    const filename =
      item.filename ??
      `generated-${item.type}-${item.taskId ?? Date.now()}.${
        item.type === "video" ? "mp4" : "png"
      }`;
    try {
      setDownloadingKey(itemKey);
      await downloadPlaygroundMedia(url, filename);
    } catch {
      toast.error(t("Download failed"));
    } finally {
      setDownloadingKey(null);
    }
  };

  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {props.items.map((item, index) => {
        const url = getSafeMediaUrl(item);
        if (!url) {
          return (
            <div
              key={`${item.type}-${item.taskId ?? index}`}
              className="border-border/70 text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border px-4 text-sm"
            >
              {item.type === "image"
                ? t("Image not available")
                : t("Video not available")}
            </div>
          );
        }

        if (item.type === "video") {
          return (
            <div
              key={`${item.type}-${item.taskId ?? item.url}`}
              className="group/media relative"
            >
              <video
                className="border-border/70 max-h-[28rem] w-full rounded-lg border bg-black"
                controls
                preload="metadata"
                src={url}
              />
              <Button
                aria-label={t("Download video")}
                className="absolute top-2 right-2 shadow-sm"
                disabled={
                  downloadingKey ===
                  (item.storageKey ?? item.taskId ?? `${item.type}-${index}`)
                }
                onClick={() => handleDownload(item, url, index)}
                size="icon-sm"
                type="button"
                variant="secondary"
              >
                <DownloadIcon />
              </Button>
            </div>
          );
        }

        return (
          <div key={`${item.type}-${url}`} className="group/media relative">
            <a href={url} rel="noreferrer" target="_blank">
              <img
                alt={t("Generated image")}
                className="border-border/70 max-h-[32rem] w-full rounded-lg border object-contain"
                loading="lazy"
                src={url}
              />
            </a>
            <Button
              aria-label={t("Download image")}
              className="absolute top-2 right-2 shadow-sm"
              disabled={
                downloadingKey ===
                (item.storageKey ?? item.taskId ?? `${item.type}-${index}`)
              }
              onClick={() => handleDownload(item, url, index)}
              size="icon-sm"
              type="button"
              variant="secondary"
            >
              <DownloadIcon />
            </Button>
          </div>
        );
      })}
    </div>
  );
}
