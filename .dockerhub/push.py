#!/usr/bin/env python3
"""Push overview, short description, and categories to Docker Hub."""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
HUB = "https://hub.docker.com/v2"


def die(msg: str, code: int = 1) -> None:
    print(msg, file=sys.stderr)
    raise SystemExit(code)


def load() -> list[tuple[dict, str]]:
    cfg = json.loads((ROOT / "config.json").read_text())
    items = cfg["repositories"] if "repositories" in cfg else [cfg]
    out: list[tuple[dict, str]] = []
    for item in items:
        readme_name = item.get("readme", "README.md")
        readme = (ROOT / readme_name).read_text()
        short = item["short_description"]
        if len(short.encode()) > 100:
            die(f"{item['repository']}: short_description is {len(short.encode())} bytes; Hub max is 100")
        if len(readme.encode()) > 25000:
            die(f"{readme_name} is {len(readme.encode())} bytes; Hub max is 25000")
        out.append((item, readme))
    return out


def request(method: str, url: str, token: str | None = None, data: object | None = None) -> tuple[int, object]:
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if token:
        headers["Authorization"] = f"JWT {token}"
    body = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read()
            parsed: object = json.loads(raw.decode()) if raw else {}
            return resp.status, parsed
    except urllib.error.HTTPError as e:
        raw = e.read().decode(errors="replace")
        try:
            parsed = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            parsed = {"text": raw}
        return e.code, parsed


def login(username: str, password: str) -> str:
    status, payload = request(
        "POST",
        f"{HUB}/users/login/",
        data={"username": username, "password": password},
    )
    if status != 200 or not isinstance(payload, dict) or not payload.get("token"):
        die(f"Docker Hub login failed ({status})")
    return str(payload["token"])


def push_one(token: str, item: dict, readme: str) -> None:
    ns, name = str(item["repository"]).split("/", 1)
    payload = {
        "description": item["short_description"],
        "full_description": readme,
    }
    status, result = request("PATCH", f"{HUB}/repositories/{ns}/{name}/", token=token, data=payload)
    if status != 200:
        die(f"PATCH {ns}/{name} failed ({status}): {result}")

    cat_status, _ = request(
        "PATCH",
        f"{HUB}/repositories/{ns}/{name}/categories/",
        token=token,
        data=item["categories"],
    )
    if cat_status != 200:
        die(f"PATCH {ns}/{name} categories failed ({cat_status})")

    status, verify = request("GET", f"{HUB}/repositories/{ns}/{name}/", token=token)
    if status != 200 or not isinstance(verify, dict):
        die(f"GET {ns}/{name} failed ({status})")
    print(f"updated {ns}/{name}")
    print(f"description: {verify.get('description')}")
    print(f"categories: {verify.get('categories')}")
    full = verify.get("full_description")
    print(f"overview_bytes: {len(full.encode()) if isinstance(full, str) else 0}")


def main() -> None:
    username = os.environ.get("DOCKERHUB_USERNAME", "").strip()
    password = os.environ.get("DOCKERHUB_TOKEN", "").strip()
    if not username or not password:
        die("DOCKERHUB_USERNAME and DOCKERHUB_TOKEN must be set")

    token = login(username, password)
    for item, readme in load():
        push_one(token, item, readme)


if __name__ == "__main__":
    main()
