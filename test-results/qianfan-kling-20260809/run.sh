#!/usr/bin/env bash

set -u

API_BASE="https://api.frimodel.com"
RESULT_DIR="${RESULT_DIR:-$(pwd)/test-results/qianfan-kling-20260809}"
IMAGE_URL="https://www.gstatic.com/webp/gallery/1.jpg"
PROMPT="A calm mountain lake at sunrise, gentle water movement, stable camera, natural colors, no text, no people."

if [[ -z "${API_KEY:-}" ]]; then
  echo "API_KEY is required" >&2
  exit 1
fi

mkdir -p "$RESULT_DIR/cases"

write_request() {
  local case_name="$1"
  local output="$2"
  case "$case_name" in
    k30_text_std)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:3,mode:"std",aspect_ratio:"16:9"}' > "$output"
      ;;
    k30_text_pro)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:3,mode:"pro",aspect_ratio:"16:9"}' > "$output"
      ;;
    k30_text_4k)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:3,mode:"4k",aspect_ratio:"16:9"}' > "$output"
      ;;
    k30_image_std)
      jq -n --arg prompt "$PROMPT" --arg image "$IMAGE_URL" '{model:"Kling 3.0",prompt:$prompt,image:$image,duration:3,mode:"std"}' > "$output"
      ;;
    k30_resolution_1080_compat)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:3,resolution:"1080p",aspect_ratio:"16:9"}' > "$output"
      ;;
    k30_invalid_resolution_999p)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:3,resolution:"999p",aspect_ratio:"16:9"}' > "$output"
      ;;
    omni_text_std)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Omni",prompt:$prompt,duration:3,mode:"std",aspect_ratio:"16:9"}' > "$output"
      ;;
    omni_text_pro)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Omni",prompt:$prompt,duration:3,mode:"pro",aspect_ratio:"16:9"}' > "$output"
      ;;
    omni_text_4k)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Omni",prompt:$prompt,duration:3,mode:"4k",aspect_ratio:"16:9"}' > "$output"
      ;;
    omni_image_std)
      jq -n --arg prompt "$PROMPT" --arg image "$IMAGE_URL" '{model:"Kling 3.0 Omni",prompt:$prompt,image:$image,duration:3,mode:"std"}' > "$output"
      ;;
    omni_resolution_720_compat)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Omni",prompt:$prompt,duration:3,resolution:"720p",aspect_ratio:"16:9"}' > "$output"
      ;;
    turbo_text_720)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Turbo",prompt:$prompt,duration:3,resolution:"720p",aspect_ratio:"16:9"}' > "$output"
      ;;
    turbo_text_1080)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Turbo",prompt:$prompt,duration:3,resolution:"1080p",aspect_ratio:"16:9"}' > "$output"
      ;;
    turbo_image_720)
      jq -n --arg prompt "$PROMPT" --arg image "$IMAGE_URL" '{model:"Kling 3.0 Turbo",prompt:$prompt,image:$image,duration:3,resolution:"720p"}' > "$output"
      ;;
    turbo_image_1080)
      jq -n --arg prompt "$PROMPT" --arg image "$IMAGE_URL" '{model:"Kling 3.0 Turbo",prompt:$prompt,image:$image,duration:3,resolution:"1080p"}' > "$output"
      ;;
    duration_2_k30)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:2,mode:"std",aspect_ratio:"16:9"}' > "$output"
      ;;
    duration_2_omni)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Omni",prompt:$prompt,duration:2,mode:"std",aspect_ratio:"16:9"}' > "$output"
      ;;
    duration_2_turbo)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Turbo",prompt:$prompt,duration:2,resolution:"720p",aspect_ratio:"16:9"}' > "$output"
      ;;
    invalid_mode_k30)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0",prompt:$prompt,duration:3,mode:"cinema",aspect_ratio:"16:9"}' > "$output"
      ;;
    invalid_resolution_turbo)
      jq -n --arg prompt "$PROMPT" '{model:"Kling 3.0 Turbo",prompt:$prompt,duration:3,resolution:"999p",aspect_ratio:"16:9"}' > "$output"
      ;;
    missing_prompt_k30)
      jq -n '{model:"Kling 3.0",duration:3,mode:"std",aspect_ratio:"16:9"}' > "$output"
      ;;
    *)
      echo "unknown case: $case_name" >&2
      return 1
      ;;
  esac
}

poll_task() {
  local case_name="$1"
  local task_id="$2"
  local case_dir="$RESULT_DIR/cases/$case_name"
  local started now elapsed status progress http_code poll_count
  started=$(date +%s)
  poll_count=0

  while true; do
    poll_count=$((poll_count + 1))
    http_code=$(curl -sS --connect-timeout 15 --max-time 60 \
      -o "$case_dir/status.json" \
      -w '%{http_code}' \
      "$API_BASE/v1/videos/$task_id" \
      -H "Authorization: Bearer $API_KEY")
    cp "$case_dir/status.json" "$case_dir/status-${poll_count}.json"
    status=$(jq -r '.status // "unknown"' "$case_dir/status.json" 2>/dev/null)
    progress=$(jq -r '.progress // "?"' "$case_dir/status.json" 2>/dev/null)
    now=$(date +%s)
    elapsed=$((now - started))
    echo "poll case=$case_name http=$http_code status=$status progress=$progress elapsed=${elapsed}s"

    if [[ "$status" == "completed" || "$status" == "failed" ]]; then
      cp "$case_dir/status.json" "$case_dir/final.json"
      break
    fi
    if (( elapsed >= ${POLL_TIMEOUT_SECONDS:-1200} )); then
      echo "poll_timeout case=$case_name"
      break
    fi
    sleep 10
  done
}

download_video() {
  local case_name="$1"
  local task_id="$2"
  local case_dir="$RESULT_DIR/cases/$case_name"
  local download_meta range_meta

  download_meta=$(curl -sS -L --connect-timeout 15 --max-time 300 \
    -D "$case_dir/content.headers" \
    -o "$case_dir/video.mp4" \
    -w 'http=%{http_code} type=%{content_type} bytes=%{size_download} total=%{time_total}' \
    "$API_BASE/v1/videos/$task_id/content" \
    -H "Authorization: Bearer $API_KEY")
  echo "download case=$case_name $download_meta"
  printf '%s\n' "$download_meta" > "$case_dir/content.meta"

  if ffprobe -v error \
    -show_entries format=duration,size,bit_rate,format_name:stream=index,codec_type,codec_name,profile,width,height,pix_fmt,r_frame_rate,avg_frame_rate,bit_rate,sample_rate,channels \
    -of json "$case_dir/video.mp4" > "$case_dir/ffprobe.json"; then
    jq -c '{format:.format,streams:.streams}' "$case_dir/ffprobe.json"
  else
    echo "ffprobe_failed case=$case_name"
  fi

  range_meta=$(curl -sS --connect-timeout 15 --max-time 90 \
    -D "$case_dir/range.headers" \
    -o "$case_dir/range.bin" \
    -w 'http=%{http_code} type=%{content_type} bytes=%{size_download}' \
    "$API_BASE/v1/videos/$task_id/content" \
    -H "Authorization: Bearer $API_KEY" \
    -H 'Range: bytes=0-1023')
  echo "range case=$case_name $range_meta"
  printf '%s\n' "$range_meta" > "$case_dir/range.meta"
}

run_case() {
  local case_name="$1"
  local poll_and_download="$2"
  local case_dir="$RESULT_DIR/cases/$case_name"
  local create_http task_id status

  mkdir -p "$case_dir"
  write_request "$case_name" "$case_dir/request.json"
  date +%s > "$case_dir/submitted_at"
  create_http=$(curl -sS --connect-timeout 15 --max-time 90 \
    -D "$case_dir/create.headers" \
    -o "$case_dir/create.json" \
    -w '%{http_code}' \
    -X POST "$API_BASE/v1/videos" \
    -H "Authorization: Bearer $API_KEY" \
    -H 'Content-Type: application/json' \
    --data-binary "@$case_dir/request.json")
  printf '%s\n' "$create_http" > "$case_dir/create.http"
  task_id=$(jq -r '.task_id // .id // ""' "$case_dir/create.json" 2>/dev/null)
  status=$(jq -r '.status // ""' "$case_dir/create.json" 2>/dev/null)
  echo "create case=$case_name http=$create_http status=$status task_id=$task_id body=$(jq -c . "$case_dir/create.json" 2>/dev/null)"

  if [[ "$poll_and_download" != "yes" || "$create_http" != "200" || -z "$task_id" ]]; then
    return
  fi

  poll_task "$case_name" "$task_id"
  status=$(jq -r '.status // ""' "$case_dir/status.json" 2>/dev/null)
  if [[ "$status" == "completed" ]]; then
    download_video "$case_name" "$task_id"
  fi
}

positive_cases=(
  k30_text_std
  k30_text_pro
  k30_text_4k
  k30_image_std
  k30_resolution_1080_compat
  k30_invalid_resolution_999p
  omni_text_std
  omni_text_pro
  omni_text_4k
  omni_image_std
  omni_resolution_720_compat
  turbo_text_720
  turbo_text_1080
  turbo_image_720
  turbo_image_1080
)

negative_cases=(
  duration_2_k30
  duration_2_omni
  duration_2_turbo
  invalid_mode_k30
  invalid_resolution_turbo
  missing_prompt_k30
)

start_case="${START_CASE:-${positive_cases[0]}}"
started=false
for case_name in "${positive_cases[@]}"; do
  if [[ "$case_name" == "$start_case" ]]; then
    started=true
  fi
  if [[ "$started" != "true" ]]; then
    continue
  fi
  run_case "$case_name" yes
done

for case_name in "${negative_cases[@]}"; do
  run_case "$case_name" no
done

echo "all_cases_submitted"
