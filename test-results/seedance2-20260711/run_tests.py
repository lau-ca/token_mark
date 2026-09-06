#!/usr/bin/env python3

import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import hashlib
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


BASE_URL = "https://newapi.megabyai.cc"
RESULTS_DIR = Path(__file__).resolve().parent
POLL_INTERVAL_SECONDS = 10
POLL_TIMEOUT_SECONDS = 30 * 60

REFERENCE_IMAGES = [
    "https://ark-project.tos-cn-beijing.volces.com/doc_image/r2v_tea_pic1.jpg",
]
REFERENCE_VIDEOS = [
    "https://ark-project.tos-cn-beijing.volces.com/doc_video/r2v_tea_video1.mp4",
]
REFERENCE_AUDIOS = [
    "https://ark-project.tos-cn-beijing.volces.com/doc_audio/r2v_tea_audio1.mp3",
]

PROMPT = (
    "以 @image1 中的主体与场景为准，参考 @video1 的动作节奏与镜头语言，"
    "并结合 @audio1 的声音节奏，生成一段镜头稳定、电影感明确的短片。"
)

CASES = {
    "videos-fast_480p_15s": {
        "billing_mode": "per_request",
        "expected_cny": 2.0,
        "request": {
            "model": "videos-fast",
            "prompt": PROMPT,
            "duration": 15,
            "ratio": "16:9",
            "resolution": "480p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-fast_720p_15s": {
        "billing_mode": "per_request",
        "expected_cny": 3.5,
        "request": {
            "model": "videos-fast",
            "prompt": PROMPT,
            "duration": 15,
            "ratio": "16:9",
            "resolution": "720p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-mini_480p_15s": {
        "billing_mode": "per_request",
        "expected_cny": 1.5,
        "request": {
            "model": "videos-mini",
            "prompt": PROMPT,
            "duration": 15,
            "ratio": "16:9",
            "resolution": "480p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-mini_720p_15s": {
        "billing_mode": "per_request",
        "expected_cny": 2.5,
        "request": {
            "model": "videos-mini",
            "prompt": PROMPT,
            "duration": 15,
            "ratio": "16:9",
            "resolution": "720p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_480p_15s": {
        "billing_mode": "per_request",
        "expected_cny": 3.5,
        "request": {
            "model": "videos-standard",
            "prompt": PROMPT,
            "duration": 15,
            "ratio": "16:9",
            "resolution": "480p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_720p_15s": {
        "billing_mode": "per_request",
        "expected_cny": 5.0,
        "request": {
            "model": "videos-standard",
            "prompt": PROMPT,
            "duration": 15,
            "ratio": "16:9",
            "resolution": "720p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_1080p_2s": {
        "billing_mode": "per_second",
        "expected_unit_cny": 0.6,
        "expected_cny": 1.2,
        "request": {
            "model": "videos-standard",
            "prompt": PROMPT,
            "duration": 2,
            "ratio": "16:9",
            "resolution": "1080p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_4k_2s": {
        "billing_mode": "per_second",
        "expected_unit_cny": 1.2,
        "expected_cny": 2.4,
        "request": {
            "model": "videos-standard",
            "prompt": PROMPT,
            "duration": 2,
            "ratio": "16:9",
            "resolution": "4k",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_1080p_4s_retry": {
        "billing_mode": "per_second",
        "expected_unit_cny": 0.6,
        "expected_cny": 2.4,
        "request": {
            "model": "videos-standard",
            "prompt": PROMPT,
            "duration": 4,
            "ratio": "16:9",
            "resolution": "1080p",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_4k_4s_retry": {
        "billing_mode": "per_second",
        "expected_unit_cny": 1.2,
        "expected_cny": 4.8,
        "request": {
            "model": "videos-standard",
            "prompt": PROMPT,
            "duration": 4,
            "ratio": "16:9",
            "resolution": "4k",
            "referenceImages": REFERENCE_IMAGES,
            "referenceVideos": REFERENCE_VIDEOS,
            "referenceAudios": REFERENCE_AUDIOS,
        },
    },
    "videos-standard_4k_4s_safe_retry": {
        "billing_mode": "per_second",
        "expected_unit_cny": 1.2,
        "expected_cny": 4.8,
        "request": {
            "model": "videos-standard",
            "prompt": "一颗红色玻璃球放在纯白背景上缓慢旋转，柔和棚拍光线，无人物、无文字、无品牌。",
            "duration": 4,
            "ratio": "16:9",
            "resolution": "4k",
        },
    },
}

TERMINAL_STATUSES = {
    "completed",
    "succeeded",
    "success",
    "failed",
    "cancelled",
    "canceled",
}


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(value, ensure_ascii=False, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def parse_json(raw_body):
    try:
        return json.loads(raw_body.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        return None


def request_json(api_key, method, path, body=None, timeout=60):
    payload = None
    headers = {
        "Accept": "application/json",
        "Authorization": f"Bearer {api_key}",
    }
    if body is not None:
        payload = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"

    request = Request(
        f"{BASE_URL}{path}",
        data=payload,
        headers=headers,
        method=method,
    )
    captured_at = now_iso()
    try:
        with urlopen(request, timeout=timeout) as response:
            raw_body = response.read()
            return {
                "captured_at": captured_at,
                "method": method,
                "url": f"{BASE_URL}{path}",
                "status_code": response.status,
                "headers": dict(response.headers.items()),
                "body": parse_json(raw_body),
                "raw_body": raw_body.decode("utf-8", errors="replace"),
            }
    except HTTPError as error:
        raw_body = error.read()
        return {
            "captured_at": captured_at,
            "method": method,
            "url": f"{BASE_URL}{path}",
            "status_code": error.code,
            "headers": dict(error.headers.items()),
            "body": parse_json(raw_body),
            "raw_body": raw_body.decode("utf-8", errors="replace"),
        }
    except URLError as error:
        return {
            "captured_at": captured_at,
            "method": method,
            "url": f"{BASE_URL}{path}",
            "status_code": None,
            "headers": {},
            "body": None,
            "raw_body": "",
            "transport_error": str(error.reason),
        }


def extract_usage(response):
    body = response.get("body")
    if not isinstance(body, dict):
        return None
    data = body.get("data")
    if not isinstance(data, dict):
        return None
    return {
        "total_granted": data.get("total_granted"),
        "total_used": data.get("total_used"),
        "total_available": data.get("total_available"),
        "unlimited_quota": data.get("unlimited_quota"),
    }


def extract_task_id(response):
    body = response.get("body")
    if not isinstance(body, dict):
        return None
    candidates = [body]
    if isinstance(body.get("data"), dict):
        candidates.append(body["data"])
    for candidate in candidates:
        for key in ("task_id", "id", "video_id"):
            value = candidate.get(key)
            if isinstance(value, str) and value:
                return value
    return None


def extract_status(response):
    body = response.get("body")
    if not isinstance(body, dict):
        return None
    candidates = [body]
    if isinstance(body.get("data"), dict):
        candidates.append(body["data"])
    for candidate in candidates:
        status = candidate.get("status")
        if isinstance(status, str) and status:
            return status.lower()
    return None


def extract_content_url(response):
    body = response.get("body")
    if not isinstance(body, dict):
        return None
    candidates = [body]
    if isinstance(body.get("data"), dict):
        candidates.append(body["data"])
    for candidate in candidates:
        for key in ("url", "video_url", "content_url"):
            value = candidate.get(key)
            if isinstance(value, str) and value.startswith(("http://", "https://")):
                return value
        metadata = candidate.get("metadata")
        if isinstance(metadata, dict):
            for key in ("content_url", "local_url", "url", "origin_video_url"):
                value = metadata.get(key)
                if isinstance(value, str) and value.startswith(("http://", "https://")):
                    return value
    return None


def download_content(api_key, task_id, case_dir):
    attempt_paths = sorted(case_dir.glob("content_download_attempt_*.json"))
    attempt_number = len(attempt_paths) + 1

    def save_download_result(result):
        write_json(
            case_dir / f"content_download_attempt_{attempt_number:04d}.json",
            result,
        )
        write_json(case_dir / "content_download.json", result)

    request = Request(
        f"{BASE_URL}/v1/videos/{task_id}/content",
        headers={"Authorization": f"Bearer {api_key}"},
        method="GET",
    )
    captured_at = now_iso()
    try:
        with urlopen(request, timeout=180) as response:
            content = response.read()
            suffix = ".mp4"
            content_type = response.headers.get("Content-Type", "")
            if "webm" in content_type:
                suffix = ".webm"
            output_path = case_dir / f"output{suffix}"
            output_path.write_bytes(content)
            metadata = {
                "captured_at": captured_at,
                "method": "GET",
                "url": f"{BASE_URL}/v1/videos/{task_id}/content",
                "status_code": response.status,
                "headers": dict(response.headers.items()),
                "saved_as": output_path.name,
                "size_bytes": len(content),
                "sha256": hashlib.sha256(content).hexdigest(),
            }
            save_download_result(metadata)
            probe = subprocess.run(
                [
                    "ffprobe",
                    "-v",
                    "error",
                    "-show_entries",
                    "stream=index,codec_type,codec_name,width,height,duration:format=duration,size,format_name",
                    "-of",
                    "json",
                    str(output_path),
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            probe_result = {
                "exit_code": probe.returncode,
                "stdout": probe.stdout,
                "stderr": probe.stderr,
            }
            if probe.returncode == 0:
                try:
                    probe_result["parsed"] = json.loads(probe.stdout)
                except json.JSONDecodeError:
                    pass
            write_json(case_dir / "output_ffprobe.json", probe_result)
            return metadata
    except HTTPError as error:
        raw_body = error.read()
        result = {
            "captured_at": captured_at,
            "method": "GET",
            "url": f"{BASE_URL}/v1/videos/{task_id}/content",
            "status_code": error.code,
            "headers": dict(error.headers.items()),
            "body": parse_json(raw_body),
            "raw_body": raw_body.decode("utf-8", errors="replace"),
        }
        save_download_result(result)
        return result
    except URLError as error:
        result = {
            "captured_at": captured_at,
            "method": "GET",
            "url": f"{BASE_URL}/v1/videos/{task_id}/content",
            "status_code": None,
            "transport_error": str(error.reason),
        }
        save_download_result(result)
        return result


def download_direct_content(url, case_dir):
    attempt_paths = sorted(case_dir.glob("direct_content_download_attempt_*.json"))
    attempt_number = len(attempt_paths) + 1

    def save_direct_result(result):
        write_json(
            case_dir / f"direct_content_download_attempt_{attempt_number:04d}.json",
            result,
        )
        write_json(case_dir / "direct_content_download.json", result)

    captured_at = now_iso()
    temp_path = case_dir / ".direct_download.tmp"
    headers_path = case_dir / ".direct_download_headers.tmp"
    curl_result = subprocess.run(
        [
            "curl",
            "--silent",
            "--show-error",
            "--http1.1",
            "--retry",
            "5",
            "--retry-delay",
            "3",
            "--retry-all-errors",
            "--connect-timeout",
            "10",
            "--max-time",
            "300",
            "--header",
            "Range: bytes=0-",
            "--dump-header",
            str(headers_path),
            "--output",
            str(temp_path),
            "--write-out",
            "%{http_code}",
            url,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    raw_headers = ""
    if headers_path.exists():
        raw_headers = headers_path.read_text(encoding="utf-8", errors="replace")
        headers_path.unlink()
    status_code = None
    try:
        status_code = int(curl_result.stdout.strip()[-3:])
    except (TypeError, ValueError):
        pass

    if curl_result.returncode == 0 and status_code in {200, 206} and temp_path.exists():
        output_path = case_dir / "output.mp4"
        os.replace(temp_path, output_path)
        digest = hashlib.sha256()
        with output_path.open("rb") as video_file:
            for chunk in iter(lambda: video_file.read(1024 * 1024), b""):
                digest.update(chunk)
        metadata = {
            "captured_at": captured_at,
            "method": "GET",
            "url": url,
            "authorization": "not sent",
            "range": "bytes=0-",
            "status_code": status_code,
            "curl_exit_code": curl_result.returncode,
            "raw_headers": raw_headers,
            "saved_as": output_path.name,
            "size_bytes": output_path.stat().st_size,
            "sha256": digest.hexdigest(),
        }
        save_direct_result(metadata)
        probe = subprocess.run(
            [
                "ffprobe",
                "-v",
                "error",
                "-show_entries",
                "stream=index,codec_type,codec_name,width,height,duration:format=duration,size,format_name",
                "-of",
                "json",
                str(output_path),
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        probe_result = {
            "exit_code": probe.returncode,
            "stdout": probe.stdout,
            "stderr": probe.stderr,
        }
        if probe.returncode == 0:
            try:
                probe_result["parsed"] = json.loads(probe.stdout)
            except json.JSONDecodeError:
                pass
        write_json(case_dir / "output_ffprobe.json", probe_result)
        return metadata

    raw_body = ""
    if temp_path.exists():
        raw_body = temp_path.read_text(encoding="utf-8", errors="replace")
        temp_path.unlink()
    result = {
        "captured_at": captured_at,
        "method": "GET",
        "url": url,
        "authorization": "not sent",
        "range": "bytes=0-",
        "status_code": status_code,
        "curl_exit_code": curl_result.returncode,
        "raw_headers": raw_headers,
        "stderr": curl_result.stderr,
        "body": parse_json(raw_body.encode("utf-8")) if raw_body else None,
        "raw_body": raw_body,
    }
    save_direct_result(result)
    return result


def download_completed_video(api_key, task_id, final_response, case_dir):
    proxy_result = download_content(api_key, task_id, case_dir)
    if proxy_result.get("status_code") == 200:
        return {"proxy": proxy_result, "direct_fallback": None}
    direct_url = extract_content_url(final_response)
    direct_result = None
    if direct_url:
        direct_result = download_direct_content(direct_url, case_dir)
    return {"proxy": proxy_result, "direct_fallback": direct_result}


def run_case(api_key, case_name):
    case = CASES[case_name]
    case_dir = RESULTS_DIR / "cases" / case_name
    case_dir.mkdir(parents=True, exist_ok=True)

    started_at = now_iso()
    input_record = {
        "case": case_name,
        "started_at": started_at,
        "endpoint": f"{BASE_URL}/v1/videos",
        "headers": {
            "Authorization": "Bearer <REDACTED>",
            "Content-Type": "application/json",
        },
        "billing_mode": case["billing_mode"],
        "expected_unit_cny": case.get("expected_unit_cny"),
        "expected_cny": case["expected_cny"],
        "body": case["request"],
    }
    write_json(case_dir / "input.json", input_record)

    usage_before_response = request_json(api_key, "GET", "/api/usage/token/")
    write_json(case_dir / "usage_before.json", usage_before_response)
    usage_before = extract_usage(usage_before_response)

    submit_response = request_json(api_key, "POST", "/v1/videos", case["request"])
    write_json(case_dir / "submit_response.json", submit_response)
    task_id = extract_task_id(submit_response)
    print(
        json.dumps(
            {
                "event": "submitted",
                "case": case_name,
                "http_status": submit_response.get("status_code"),
                "task_id": task_id,
                "status": extract_status(submit_response),
            },
            ensure_ascii=False,
        ),
        flush=True,
    )

    final_response = submit_response
    poll_count = 0
    if task_id:
        deadline = time.monotonic() + POLL_TIMEOUT_SECONDS
        while time.monotonic() < deadline:
            status = extract_status(final_response)
            if status in TERMINAL_STATUSES:
                break
            time.sleep(POLL_INTERVAL_SECONDS)
            poll_count += 1
            final_response = request_json(api_key, "GET", f"/v1/videos/{task_id}")
            write_json(case_dir / f"poll_{poll_count:04d}.json", final_response)
            print(
                json.dumps(
                    {
                        "event": "poll",
                        "case": case_name,
                        "poll": poll_count,
                        "http_status": final_response.get("status_code"),
                        "status": extract_status(final_response),
                    },
                    ensure_ascii=False,
                ),
                flush=True,
            )

    write_json(case_dir / "final_response.json", final_response)

    content_result = None
    final_status = extract_status(final_response)
    if task_id and final_status in {"completed", "succeeded", "success"}:
        content_result = download_completed_video(
            api_key, task_id, final_response, case_dir
        )

    time.sleep(2)
    usage_after_response = request_json(api_key, "GET", "/api/usage/token/")
    write_json(case_dir / "usage_after.json", usage_after_response)
    usage_after = extract_usage(usage_after_response)

    quota_delta = None
    if usage_before and usage_after:
        before_used = usage_before.get("total_used")
        after_used = usage_after.get("total_used")
        if isinstance(before_used, (int, float)) and isinstance(after_used, (int, float)):
            quota_delta = after_used - before_used

    summary = {
        "case": case_name,
        "started_at": started_at,
        "finished_at": now_iso(),
        "request": case["request"],
        "billing_mode": case["billing_mode"],
        "expected_unit_cny": case.get("expected_unit_cny"),
        "expected_cny": case["expected_cny"],
        "submit_http_status": submit_response.get("status_code"),
        "task_id": task_id,
        "final_status": final_status,
        "poll_count": poll_count,
        "usage_before": usage_before,
        "usage_after": usage_after,
        "observed_quota_delta": quota_delta,
        "content_download": content_result,
    }
    write_json(case_dir / "summary.json", summary)
    print(json.dumps({"event": "finished", **summary}, ensure_ascii=False), flush=True)
    return summary


def resume_case(api_key, case_name):
    case_dir = RESULTS_DIR / "cases" / case_name
    submit_path = case_dir / "submit_response.json"
    if not submit_path.exists():
        raise RuntimeError(f"missing submit response for {case_name}")

    submit_response = read_json(submit_path)
    task_id = extract_task_id(submit_response)
    if not task_id:
        raise RuntimeError(f"missing task id for {case_name}")

    poll_paths = sorted(case_dir.glob("poll_*.json"))
    poll_count = int(poll_paths[-1].stem.split("_")[-1]) if poll_paths else 0
    final_response = read_json(poll_paths[-1]) if poll_paths else submit_response
    resumed_at = now_iso()
    deadline = time.monotonic() + POLL_TIMEOUT_SECONDS

    while time.monotonic() < deadline:
        status = extract_status(final_response)
        if status in TERMINAL_STATUSES:
            break
        time.sleep(POLL_INTERVAL_SECONDS)
        poll_count += 1
        final_response = request_json(api_key, "GET", f"/v1/videos/{task_id}")
        write_json(case_dir / f"poll_{poll_count:04d}.json", final_response)
        print(
            json.dumps(
                {
                    "event": "poll",
                    "case": case_name,
                    "poll": poll_count,
                    "http_status": final_response.get("status_code"),
                    "status": extract_status(final_response),
                },
                ensure_ascii=False,
            ),
            flush=True,
        )

    write_json(case_dir / "final_response.json", final_response)
    final_status = extract_status(final_response)
    content_result = None
    if final_status in {"completed", "succeeded", "success"}:
        content_result = download_completed_video(
            api_key, task_id, final_response, case_dir
        )

    usage_after_response = request_json(api_key, "GET", "/api/usage/token/")
    write_json(case_dir / "usage_after_resume.json", usage_after_response)
    summary_path = case_dir / "summary.json"
    summary = read_json(summary_path) if summary_path.exists() else {
        "case": case_name,
        "request": CASES[case_name]["request"],
        "billing_mode": CASES[case_name]["billing_mode"],
        "expected_unit_cny": CASES[case_name].get("expected_unit_cny"),
        "expected_cny": CASES[case_name]["expected_cny"],
        "submit_http_status": submit_response.get("status_code"),
        "task_id": task_id,
    }
    summary.update(
        {
            "resumed_at": resumed_at,
            "resume_finished_at": now_iso(),
            "final_status": final_status,
            "poll_count": poll_count,
            "content_download": content_result,
            "usage_after_resume": extract_usage(usage_after_response),
        }
    )
    write_json(summary_path, summary)
    print(json.dumps({"event": "finished", **summary}, ensure_ascii=False), flush=True)
    return summary


def write_manifest():
    manifest = {
        "created_at": now_iso(),
        "base_url": BASE_URL,
        "authorization": "Bearer <REDACTED>",
        "poll_interval_seconds": POLL_INTERVAL_SECONDS,
        "poll_timeout_seconds": POLL_TIMEOUT_SECONDS,
        "documentation": {
            "compatibility_api": "https://4os673ec2p.apifox.cn/472626689e0",
            "volcengine_api_reference": "https://www.volcengine.com/docs/82379/1520757",
            "volcengine_seedance_2_tutorial": "https://www.volcengine.com/docs/82379/2291680",
        },
        "reference_assets": {
            "source_note": "Volcengine official Seedance 2.0 tutorial media",
            "images": REFERENCE_IMAGES,
            "videos": REFERENCE_VIDEOS,
            "audios": REFERENCE_AUDIOS,
        },
        "cases": CASES,
    }
    write_json(RESULTS_DIR / "manifest.json", manifest)


def capture_environment(api_key):
    write_json(
        RESULTS_DIR / "environment_models.json",
        request_json(api_key, "GET", "/v1/models"),
    )
    write_json(
        RESULTS_DIR / "environment_status.json",
        request_json(api_key, "GET", "/api/status"),
    )
    write_json(
        RESULTS_DIR / "environment_usage_initial.json",
        request_json(api_key, "GET", "/api/usage/token/"),
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--case", choices=CASES.keys())
    parser.add_argument("--all", action="store_true")
    parser.add_argument("--parallel", action="store_true")
    parser.add_argument("--skip-existing", action="store_true")
    parser.add_argument("--resume-existing", action="store_true")
    args = parser.parse_args()

    if not args.case and not args.all:
        parser.error("use --case CASE or --all")

    api_key = os.environ.get("NEWAPI_API_KEY")
    if not api_key:
        print("NEWAPI_API_KEY is required", file=sys.stderr)
        return 2

    write_manifest()
    capture_environment(api_key)
    selected_cases = list(CASES) if args.all else [args.case]
    if args.skip_existing:
        selected_cases = [
            case_name
            for case_name in selected_cases
            if not (
                RESULTS_DIR / "cases" / case_name / "submit_response.json"
            ).exists()
        ]
    summaries = []
    worker = resume_case if args.resume_existing else run_case
    if args.parallel and len(selected_cases) > 1:
        with ThreadPoolExecutor(max_workers=len(selected_cases)) as executor:
            future_to_case = {
                executor.submit(worker, api_key, case_name): case_name
                for case_name in selected_cases
            }
            for future in as_completed(future_to_case):
                summaries.append(future.result())
    else:
        for case_name in selected_cases:
            summaries.append(worker(api_key, case_name))

    summary_path = RESULTS_DIR / "run_summary.json"
    merged_summaries = {}
    if summary_path.exists():
        try:
            existing = json.loads(summary_path.read_text(encoding="utf-8"))
            for summary in existing.get("cases", []):
                if isinstance(summary, dict) and summary.get("case"):
                    merged_summaries[summary["case"]] = summary
        except (OSError, json.JSONDecodeError):
            pass
    for summary in summaries:
        merged_summaries[summary["case"]] = summary
    ordered_summaries = [
        merged_summaries[case_name]
        for case_name in CASES
        if case_name in merged_summaries
    ]
    write_json(
        summary_path,
        {
            "finished_at": now_iso(),
            "cases": ordered_summaries,
        },
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
