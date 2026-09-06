#!/bin/bash
set -e
run=/tmp/direct-channel193-$(date +%s)
mkdir -p "$run"
k=$(sudo docker exec component-postgres psql -U Aether -d gateway -Atc "select key from channels where id=193" | head -1)
printf '%s' '{"model":"gpt-image-2","prompt":"A cinematic ultra-detailed futuristic Shanghai skyline at night viewed from a rooftop after heavy rain, vast panoramic cityscape stretching to the horizon, intricate realistic architecture, thousands of illuminated windows, neon reflections on rain-soaked streets, elegant flying vehicles moving through layered atmospheric fog, highly detailed foreground rooftop equipment and wet textures, realistic volumetric lighting, dramatic blue and magenta color contrast, premium editorial photography style, natural perspective, physically accurate materials, subtle film grain, exceptional detail, no text, no logos, no watermark, no border, no distortion, no duplicated objects, no artifacts.","size":"3840x2160","response_format":"b64_json"}' > "$run/payload.json"
for i in $(seq 1 20); do
  curl --http1.1 --max-time 180 -sS -o "$run/$i.body" -w "REQ=$i HTTP=%{http_code} TIME=%{time_total} SIZE=%{size_download}\n" "https://llmway.ai/v1/images/generations" -H "Authorization: Bearer $k" -H 'Content-Type: application/json' --data-binary @"$run/payload.json" > "$run/$i.meta" 2>&1 &
done
wait
echo RUN=$run
cat "$run"/*.meta | sort
