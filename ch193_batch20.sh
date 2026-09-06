#!/bin/bash
set -e
run=/tmp/ch193-batch-$(date +%s)
mkdir -p "$run"
k=$(sudo docker exec component-postgres psql -U Aether -d gateway -Atc "select key from channels where id=193" | head -1)
printf '%s' '{"model":"gpt-image-2","prompt":"A simple test image of a blue circle on a white background, no text.","size":"1024x1024","response_format":"b64_json"}' > "$run/payload.json"
for i in $(seq 1 20); do
  curl --http1.1 --max-time 120 -sS -o "$run/$i.body" -w "HTTP=%{http_code} TIME=%{time_total} SIZE=%{size_download}\n" "https://llmway.ai/v1/images/generations" -H "Authorization: Bearer $k" -H 'Content-Type: application/json' --data-binary @"$run/payload.json" > "$run/$i.meta" 2>&1 &
done
wait
echo RUN=$run
cat "$run"/*.meta | sort
