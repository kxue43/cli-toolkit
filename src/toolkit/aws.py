# Builtin
from __future__ import annotations
from datetime import datetime, timedelta, timezone
import hashlib
import json
from pathlib import Path
import re
from typing import List, Tuple, TypedDict


CACHE_DIR = Path.home().joinpath(".aws", "toolkit-cache")


# Type Checking
class CredentialProcessOutput(TypedDict):
    Version: int
    AccessKeyId: str
    SecretAccessKey: str
    SessionToken: str
    Expiration: str


def _get_prefix(role_arn: str) -> str:
    m = hashlib.sha1()
    m.update(role_arn.encode("utf-8"))
    return m.hexdigest()[0:7]


def save_cache(role_arn: str, d: CredentialProcessOutput) -> None:
    if CACHE_DIR.exists() and CACHE_DIR.is_file():
        CACHE_DIR.unlink()
    if not CACHE_DIR.exists():
        CACHE_DIR.mkdir(parents=True)
    prefix = _get_prefix(role_arn)
    ts = datetime.fromisoformat(d["Expiration"]).timestamp()
    file_path = CACHE_DIR.joinpath(f"{prefix}-{round(ts)}.json")
    with open(file_path, "w") as fw:
        json.dump(d, fw)


def get_active_from_cache(role_arn: str) -> str | None:
    if not (CACHE_DIR.exists() and CACHE_DIR.is_dir()):
        return None
    prefix = _get_prefix(role_arn)
    regex = re.compile(rf"^{prefix}-(\d+)\.json$")
    now = datetime.now(timezone.utc)
    active: List[Tuple[datetime, Path]] = []
    for item in CACHE_DIR.glob(f"{prefix}-*.json"):
        match_ = regex.match(item.name)
        if match_ is None:
            item.unlink()
            continue
        expire = datetime.fromtimestamp(float(match_.group(1)), timezone.utc)
        if expire - now < timedelta(minutes=15):
            item.unlink()
            continue
        active.append((expire, item))
    if len(active) == 0:
        return None
    active.sort(key=lambda x: x[0], reverse=True)
    for _, item in active[1:]:
        item.unlink()
    with open(active[0][1], "r") as fr:
        return fr.read()
