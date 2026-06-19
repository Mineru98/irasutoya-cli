#!/usr/bin/env python3
"""Run irasutoya CLI searches with a verified release fallback."""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import platform
import re
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Sequence


DEFAULT_REPO = "Mineru98/irasutoya-cli"
SCRIPT_DIR = Path(__file__).resolve().parent
BIN_DIR = SCRIPT_DIR / "bin"
USER_AGENT = "irasutoya-search-skill/1.0"


class IrasutoyaSearchError(RuntimeError):
    """Actionable error surfaced to the caller."""


@dataclass(frozen=True)
class Target:
    goos: str
    goarch: str
    archive_ext: str
    exe_name: str

    @property
    def cache_key(self) -> str:
        return f"{self.goos}_{self.goarch}"


@dataclass(frozen=True)
class Asset:
    name: str
    browser_download_url: str


@dataclass(frozen=True)
class Release:
    tag_name: str
    assets: tuple[Asset, ...]


def normalize_target(system: str | None = None, machine: str | None = None) -> Target:
    system_value = (system or platform.system()).lower()
    machine_value = (machine or platform.machine()).lower()
    goos_map = {"windows": "windows", "linux": "linux", "darwin": "darwin"}
    arch_map = {"amd64": "amd64", "x86_64": "amd64", "x64": "amd64", "arm64": "arm64", "aarch64": "arm64"}

    if system_value not in goos_map:
        raise IrasutoyaSearchError(f"unsupported platform: {system or platform.system()}")
    if machine_value not in arch_map:
        raise IrasutoyaSearchError(f"unsupported architecture: {machine or platform.machine()}")

    goos = goos_map[system_value]
    goarch = arch_map[machine_value]
    return Target(goos, goarch, ".zip" if goos == "windows" else ".tar.gz", "irasutoya.exe" if goos == "windows" else "irasutoya")


def ensure_https(url: str, label: str) -> None:
    if not url.startswith("https://"):
        raise IrasutoyaSearchError(f"{label} must use https: {url}")


def github_api_json(url: str) -> dict:
    ensure_https(url, "GitHub API URL")
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT, "Accept": "application/vnd.github+json"})
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        if exc.code == 404:
            raise IrasutoyaSearchError("release fallback live validation blocked: no published release") from exc
        raise IrasutoyaSearchError(f"GitHub release API request failed: HTTP {exc.code}") from exc
    except urllib.error.URLError as exc:
        raise IrasutoyaSearchError(f"GitHub release API request failed: {exc.reason}") from exc


def latest_release(repo: str = DEFAULT_REPO) -> Release:
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
        raise IrasutoyaSearchError(f"invalid GitHub repo name: {repo}")
    data = github_api_json(f"https://api.github.com/repos/{repo}/releases/latest")
    assets = tuple(
        Asset(str(asset.get("name", "")), str(asset.get("browser_download_url", "")))
        for asset in data.get("assets", [])
    )
    tag_name = str(data.get("tag_name", "")).strip()
    if not tag_name:
        raise IrasutoyaSearchError("latest release response did not include tag_name")
    if not assets:
        raise IrasutoyaSearchError(f"latest release {tag_name} has no assets")
    return Release(tag_name, assets)


def select_archive_asset(release: Release, target: Target) -> Asset:
    pattern = re.compile(
        rf"^irasutoya(?:[_-].+)?[_-]{re.escape(target.goos)}[_-]{re.escape(target.goarch)}{re.escape(target.archive_ext)}$"
    )
    matches = [asset for asset in release.assets if pattern.match(asset.name)]
    if not matches:
        names = ", ".join(asset.name for asset in release.assets) or "<none>"
        raise IrasutoyaSearchError(f"no release asset matches {target.goos}/{target.goarch} ({target.archive_ext}); assets: {names}")
    if len(matches) > 1:
        raise IrasutoyaSearchError("multiple matching release assets: " + ", ".join(asset.name for asset in matches))
    ensure_https(matches[0].browser_download_url, "release asset URL")
    return matches[0]


def select_checksum_asset(release: Release) -> Asset:
    matches = [asset for asset in release.assets if asset.name == "checksums.txt"]
    if not matches:
        raise IrasutoyaSearchError("latest release does not include exact checksums.txt asset")
    if len(matches) > 1:
        raise IrasutoyaSearchError("latest release includes multiple checksums.txt assets")
    ensure_https(matches[0].browser_download_url, "checksum asset URL")
    return matches[0]


def download_bytes(url: str) -> bytes:
    ensure_https(url, "download URL")
    request = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            return response.read()
    except urllib.error.HTTPError as exc:
        raise IrasutoyaSearchError(f"download failed: HTTP {exc.code} for {url}") from exc
    except urllib.error.URLError as exc:
        raise IrasutoyaSearchError(f"download failed: {exc.reason}") from exc


def expected_sha256(checksums_text: str, asset_name: str) -> str:
    for raw_line in checksums_text.splitlines():
        parts = raw_line.strip().split()
        if len(parts) != 2:
            continue
        digest, filename = parts
        if filename.lstrip("*") == asset_name:
            if not re.fullmatch(r"[0-9a-fA-F]{64}", digest):
                raise IrasutoyaSearchError(f"invalid sha256 digest in checksums.txt for {asset_name}")
            return digest.lower()
    raise IrasutoyaSearchError(f"checksums.txt has no entry for {asset_name}")


def verify_sha256(content: bytes, expected_digest: str, asset_name: str) -> None:
    actual = hashlib.sha256(content).hexdigest()
    if actual != expected_digest:
        raise IrasutoyaSearchError(f"checksum mismatch for {asset_name}: expected {expected_digest}, got {actual}")


def assert_within(path: Path, base: Path) -> Path:
    resolved_path = path.resolve()
    resolved_base = base.resolve()
    if resolved_path != resolved_base and resolved_base not in resolved_path.parents:
        raise IrasutoyaSearchError(f"path escapes cache directory: {path}")
    return resolved_path


def safe_extract_zip(content: bytes, destination: Path) -> None:
    with zipfile.ZipFile(io.BytesIO(content)) as archive:
        for info in archive.infolist():
            assert_within(destination / info.filename, destination)
            mode = (info.external_attr >> 16) & 0o777777
            if stat.S_ISLNK(mode):
                raise IrasutoyaSearchError(f"zip archive contains unsupported symlink: {info.filename}")
        archive.extractall(destination)


def safe_extract_tar(content: bytes, destination: Path) -> None:
    with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
        for member in archive.getmembers():
            if member.issym() or member.islnk():
                raise IrasutoyaSearchError(f"tar archive contains unsupported link: {member.name}")
            if not (member.isfile() or member.isdir()):
                raise IrasutoyaSearchError(f"tar archive contains unsupported member type: {member.name}")
            assert_within(destination / member.name, destination)
        archive.extractall(destination)


def find_executable(root: Path, target: Target) -> Path:
    matches = sorted((path for path in root.rglob(target.exe_name) if path.is_file()), key=lambda path: len(path.parts))
    if not matches:
        raise IrasutoyaSearchError(f"archive did not contain {target.exe_name}")
    exe = assert_within(matches[0], root)
    if target.goos != "windows":
        exe.chmod(exe.stat().st_mode | stat.S_IXUSR)
    return exe


def cache_root_for(release: Release, target: Target) -> Path:
    safe_tag = re.sub(r"[^A-Za-z0-9_.-]", "_", release.tag_name)
    return BIN_DIR / safe_tag / target.cache_key


def cached_candidates(target: Target) -> list[Path]:
    if not BIN_DIR.exists():
        return []
    candidates: list[Path] = []
    for tag_dir in BIN_DIR.iterdir():
        cache_dir = tag_dir / target.cache_key
        exe = cache_dir / target.exe_name
        marker = cache_dir / ".verified.json"
        if exe.is_file() and marker.is_file():
            candidates.append(assert_within(exe, BIN_DIR))
    return sorted(candidates, key=lambda path: path.stat().st_mtime, reverse=True)


def download_release_binary(repo: str, target: Target) -> Path:
    release = latest_release(repo)
    destination = cache_root_for(release, target)
    cached = destination / target.exe_name
    if cached.is_file():
        return assert_within(cached, destination)

    archive_asset = select_archive_asset(release, target)
    checksum_asset = select_checksum_asset(release)
    archive_bytes = download_bytes(archive_asset.browser_download_url)
    checksums_text = download_bytes(checksum_asset.browser_download_url).decode("utf-8")
    archive_digest = expected_sha256(checksums_text, archive_asset.name)
    verify_sha256(archive_bytes, archive_digest, archive_asset.name)

    with tempfile.TemporaryDirectory(prefix="irasutoya-extract-") as temp_name:
        temp_dir = Path(temp_name)
        if target.archive_ext == ".zip":
            safe_extract_zip(archive_bytes, temp_dir)
        else:
            safe_extract_tar(archive_bytes, temp_dir)
        exe = find_executable(temp_dir, target)
        destination.parent.mkdir(parents=True, exist_ok=True)
        if destination.exists():
            shutil.rmtree(destination)
        shutil.copytree(temp_dir, destination)
        marker = destination / ".verified.json"
        marker.write_text(
            json.dumps(
                {
                    "tag": release.tag_name,
                    "target": target.cache_key,
                    "asset": archive_asset.name,
                    "sha256": archive_digest,
                },
                indent=2,
                sort_keys=True,
            ),
            encoding="utf-8",
        )
        return assert_within(destination / exe.relative_to(temp_dir), destination)


def resolve_binary(explicit_binary: str | None, repo: str, no_download: bool = False) -> Path:
    if explicit_binary:
        path = Path(explicit_binary).expanduser().resolve()
        if not path.is_file():
            raise IrasutoyaSearchError(f"explicit binary does not exist: {path}")
        return path

    path_binary = shutil.which("irasutoya")
    if path_binary:
        return Path(path_binary).resolve()

    target = normalize_target()
    candidates = cached_candidates(target)
    if candidates:
        return candidates[0]

    if no_download:
        raise IrasutoyaSearchError("irasutoya binary not found on PATH or in cache; download disabled")
    return download_release_binary(repo, target)


def parse_results(output: str) -> list[dict[str, object]]:
    results: list[dict[str, object]] = []
    current: dict[str, object] = {}
    field_map = {"Page URL": "page_url", "Title": "title", "Description": "description", "Image URL": "image_urls"}
    for raw_line in output.splitlines():
        line = raw_line.rstrip()
        if not line:
            if current:
                results.append(current)
                current = {}
            continue
        if ":" not in line:
            continue
        raw_key, raw_value = line.split(":", 1)
        mapped = field_map.get(raw_key.strip())
        value = raw_value.strip()
        if mapped == "image_urls":
            current.setdefault("image_urls", [])
            assert isinstance(current["image_urls"], list)
            current["image_urls"].append(value)
        elif mapped:
            current[mapped] = value
    if current:
        results.append(current)
    return results


def summarize(binary: Path, query: str, results: Sequence[dict[str, object]], opened_images: bool, max_images: int | None) -> str:
    lines = [f"Binary: {binary}", f"Query: {query}", f"Image opening: {'enabled' if opened_images else 'disabled'}", f"Results: {len(results)}"]
    for index, result in enumerate(results, start=1):
        lines.extend(["", f"[{index}] {result.get('title', '<no title>')}"])
        if result.get("page_url"):
            lines.append(f"Page URL: {result['page_url']}")
        if result.get("description"):
            lines.append(f"Description: {result['description']}")
        image_urls = result.get("image_urls")
        if isinstance(image_urls, list):
            visible_urls = image_urls if max_images is None else image_urls[:max_images]
            for image_url in visible_urls:
                lines.append(f"Image URL: {image_url}")
            if max_images is not None and len(image_urls) > max_images:
                lines.append(f"Image URLs omitted: {len(image_urls) - max_images} (use --all-images to show all)")
    return "\n".join(lines)


def run_search(binary: Path, query: str, open_images: bool) -> tuple[str, list[dict[str, object]]]:
    command = [str(binary)]
    if open_images:
        command.append("--open-images")
    command.extend(["search", query])
    try:
        completed = subprocess.run(command, text=True, encoding="utf-8", errors="replace", capture_output=True, check=False, timeout=120)
    except OSError as exc:
        raise IrasutoyaSearchError(f"failed to execute {binary}: {exc}") from exc
    except subprocess.TimeoutExpired as exc:
        raise IrasutoyaSearchError(f"irasutoya search timed out for query: {query}") from exc
    if completed.returncode != 0:
        stderr = completed.stderr.strip() or completed.stdout.strip()
        raise IrasutoyaSearchError(f"irasutoya search failed with exit code {completed.returncode}: {stderr}")
    results = parse_results(completed.stdout)
    if not results:
        raise IrasutoyaSearchError("irasutoya search produced no parseable results")
    return completed.stdout, results


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Search Irasutoya through the irasutoya CLI.")
    parser.add_argument("query", nargs="+", help="Search query.")
    parser.add_argument("--open-images", action="store_true", help="Pass --open-images to the CLI.")
    parser.add_argument("--binary", help="Use this irasutoya binary instead of PATH/cache/release resolution.")
    parser.add_argument("--repo", default=DEFAULT_REPO, help=f"GitHub repo for release fallback (default: {DEFAULT_REPO}).")
    parser.add_argument("--no-download", action="store_true", help="Do not download release assets when no binary/cache exists.")
    parser.add_argument("--json", action="store_true", help="Emit structured JSON instead of a readable summary.")
    parser.add_argument("--all-images", action="store_true", help="Show every parsed image URL in the readable summary.")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")
    args = build_parser().parse_args(argv)
    query = " ".join(args.query).strip()
    try:
        binary = resolve_binary(args.binary, args.repo, no_download=args.no_download)
        _, results = run_search(binary, query, args.open_images)
        if args.json:
            print(json.dumps({"binary": str(binary), "query": query, "open_images": args.open_images, "results": results}, ensure_ascii=False, indent=2))
        else:
            print(summarize(binary, query, results, args.open_images, None if args.all_images else 5))
        return 0
    except IrasutoyaSearchError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
