#!/usr/bin/env python3
"""Deterministic tests for irasutoya_search.py."""

from __future__ import annotations

import hashlib
import io
import os
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest import mock

import irasutoya_search as subject


def release_with(*names: str) -> subject.Release:
    return subject.Release(
        "v1.2.3",
        tuple(subject.Asset(name, f"https://example.test/{name}") for name in names),
    )


def write_valid_marker(cache_dir: Path, exe: Path, target: subject.Target) -> None:
    subject.write_verified_marker(
        cache_dir,
        subject.Release("v1.2.3", ()),
        target,
        f"irasutoya_v1.2.3_{target.cache_key}{target.archive_ext}",
        "0" * 64,
        exe,
    )


class TargetTests(unittest.TestCase):
    def test_os_arch_mapping(self) -> None:
        cases = [
            ("Windows", "AMD64", "windows", "amd64", ".zip"),
            ("Windows", "ARM64", "windows", "arm64", ".zip"),
            ("Linux", "x86_64", "linux", "amd64", ".tar.gz"),
            ("Linux", "aarch64", "linux", "arm64", ".tar.gz"),
            ("Darwin", "amd64", "darwin", "amd64", ".tar.gz"),
            ("Darwin", "arm64", "darwin", "arm64", ".tar.gz"),
        ]
        for system, machine, goos, goarch, archive_ext in cases:
            with self.subTest(system=system, machine=machine):
                target = subject.normalize_target(system, machine)
                self.assertEqual((target.goos, target.goarch, target.archive_ext), (goos, goarch, archive_ext))


class AssetSelectionTests(unittest.TestCase):
    def test_select_archive_and_checksum_assets(self) -> None:
        release = release_with("irasutoya_v1.2.3_windows_amd64.zip", "checksums.txt")
        target = subject.Target("windows", "amd64", ".zip", "irasutoya.exe")
        self.assertEqual(subject.select_archive_asset(release, target).name, "irasutoya_v1.2.3_windows_amd64.zip")
        self.assertEqual(subject.select_checksum_asset(release).name, "checksums.txt")

    def test_rejects_non_https_asset_url(self) -> None:
        release = subject.Release("v1.2.3", (subject.Asset("irasutoya_v1.2.3_linux_amd64.tar.gz", "http://example.test/archive"),))
        with self.assertRaisesRegex(subject.IrasutoyaSearchError, "https"):
            subject.select_archive_asset(release, subject.Target("linux", "amd64", ".tar.gz", "irasutoya"))

    def test_requires_exact_checksum_asset(self) -> None:
        release = release_with("irasutoya_v1.2.3_linux_amd64.tar.gz", "other-checksums.txt")
        with self.assertRaisesRegex(subject.IrasutoyaSearchError, "checksums.txt"):
            subject.select_checksum_asset(release)


class ChecksumTests(unittest.TestCase):
    def test_checksum_success_and_failure(self) -> None:
        content = b"archive"
        digest = hashlib.sha256(content).hexdigest()
        checksums = f"{digest}  irasutoya_v1.2.3_linux_amd64.tar.gz\n"
        expected = subject.expected_sha256(checksums, "irasutoya_v1.2.3_linux_amd64.tar.gz")
        subject.verify_sha256(content, expected, "irasutoya_v1.2.3_linux_amd64.tar.gz")
        with self.assertRaisesRegex(subject.IrasutoyaSearchError, "checksum mismatch"):
            subject.verify_sha256(b"changed", expected, "irasutoya_v1.2.3_linux_amd64.tar.gz")


class GitHubReleaseTests(unittest.TestCase):
    def test_github_404_reports_no_published_release_blocker(self) -> None:
        error = subject.urllib.error.HTTPError(
            "https://api.github.com/repos/Mineru98/irasutoya-cli/releases/latest",
            404,
            "Not Found",
            {},
            None,
        )
        with mock.patch.object(subject.urllib.request, "urlopen", side_effect=error):
            with self.assertRaisesRegex(subject.IrasutoyaSearchError, "no published release"):
                subject.latest_release(subject.DEFAULT_REPO)


class ExtractionTests(unittest.TestCase):
    def test_zip_path_traversal_is_rejected(self) -> None:
        data = io.BytesIO()
        with zipfile.ZipFile(data, "w") as archive:
            archive.writestr("../escape.txt", "bad")
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(subject.IrasutoyaSearchError, "escapes"):
                subject.safe_extract_zip(data.getvalue(), Path(tmp))

    def test_tar_path_traversal_is_rejected(self) -> None:
        data = io.BytesIO()
        with tarfile.open(fileobj=data, mode="w:gz") as archive:
            payload = b"bad"
            info = tarfile.TarInfo("../escape.txt")
            info.size = len(payload)
            archive.addfile(info, io.BytesIO(payload))
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(subject.IrasutoyaSearchError, "escapes"):
                subject.safe_extract_tar(data.getvalue(), Path(tmp))

    def test_tar_special_member_is_rejected(self) -> None:
        data = io.BytesIO()
        with tarfile.open(fileobj=data, mode="w:gz") as archive:
            info = tarfile.TarInfo("special")
            info.type = tarfile.FIFOTYPE
            archive.addfile(info)
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(subject.IrasutoyaSearchError, "unsupported member type"):
                subject.safe_extract_tar(data.getvalue(), Path(tmp))


class CachePathSafetyTests(unittest.TestCase):
    def test_rejects_symlinked_bin_dir(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            real_bin = root / "real-bin"
            real_bin.mkdir()
            linked_bin = root / "bin"
            try:
                linked_bin.symlink_to(real_bin, target_is_directory=True)
            except (OSError, NotImplementedError) as exc:
                self.skipTest(f"directory symlink unavailable: {exc}")
            with mock.patch.object(subject, "BIN_DIR", linked_bin):
                with self.assertRaisesRegex(subject.IrasutoyaSearchError, "symlink or junction"):
                    subject.assert_safe_cache_path(linked_bin / "v1.2.3")

    def test_rejects_symlinked_cache_component(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cache_root = Path(tmp) / "bin"
            cache_root.mkdir()
            real_tag = cache_root / "real-tag"
            real_tag.mkdir()
            linked_tag = cache_root / "v1.2.3"
            try:
                linked_tag.symlink_to(real_tag, target_is_directory=True)
            except (OSError, NotImplementedError) as exc:
                self.skipTest(f"directory symlink unavailable: {exc}")
            with mock.patch.object(subject, "BIN_DIR", cache_root):
                with self.assertRaisesRegex(subject.IrasutoyaSearchError, "symlink or junction"):
                    subject.assert_safe_cache_path(linked_tag / "linux_amd64")


class ResolutionTests(unittest.TestCase):
    def test_path_first_resolution(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            exe = Path(tmp) / ("irasutoya.exe" if os.name == "nt" else "irasutoya")
            exe.write_text("", encoding="utf-8")
            with mock.patch.object(subject.shutil, "which", return_value=str(exe)), mock.patch.object(
                subject, "download_release_binary", side_effect=AssertionError("download should not run")
            ):
                self.assertEqual(subject.resolve_binary(None, subject.DEFAULT_REPO), exe.resolve())

    def test_no_download_error_when_missing(self) -> None:
        with mock.patch.object(subject.shutil, "which", return_value=None), mock.patch.object(subject, "cached_candidates", return_value=[]):
            with self.assertRaisesRegex(subject.IrasutoyaSearchError, "download disabled"):
                subject.resolve_binary(None, subject.DEFAULT_REPO, no_download=True)

    def test_cache_key_is_tag_and_platform(self) -> None:
        release = release_with("irasutoya_v1.2.3_linux_amd64.tar.gz", "checksums.txt")
        root = subject.cache_root_for(release, subject.Target("linux", "amd64", ".tar.gz", "irasutoya"))
        self.assertTrue(str(root).endswith(str(Path("v1.2.3") / "linux_amd64")))

    def test_cached_candidates_require_verified_marker(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cache_root = Path(tmp) / "bin"
            cache_dir = cache_root / "v1.2.3" / "linux_amd64"
            cache_dir.mkdir(parents=True)
            exe = cache_dir / "irasutoya"
            exe.write_text("", encoding="utf-8")
            target = subject.Target("linux", "amd64", ".tar.gz", "irasutoya")
            with mock.patch.object(subject, "BIN_DIR", cache_root):
                self.assertEqual(subject.cached_candidates(target), [])
                (cache_dir / ".verified.json").write_text("{}", encoding="utf-8")
                self.assertEqual(subject.cached_candidates(target), [])
                write_valid_marker(cache_dir, exe, target)
                self.assertEqual(subject.cached_candidates(target), [exe.resolve()])

    def test_cached_candidates_reject_tampered_executable(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cache_root = Path(tmp) / "bin"
            cache_dir = cache_root / "v1.2.3" / "linux_amd64"
            cache_dir.mkdir(parents=True)
            exe = cache_dir / "irasutoya"
            exe.write_text("original", encoding="utf-8")
            target = subject.Target("linux", "amd64", ".tar.gz", "irasutoya")
            with mock.patch.object(subject, "BIN_DIR", cache_root):
                write_valid_marker(cache_dir, exe, target)
                exe.write_text("tampered", encoding="utf-8")
                self.assertEqual(subject.cached_candidates(target), [])

    def test_download_release_binary_reuses_only_verified_cache(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cache_root = Path(tmp) / "bin"
            target = subject.Target("linux", "amd64", ".tar.gz", "irasutoya")
            release = release_with("irasutoya_v1.2.3_linux_amd64.tar.gz", "checksums.txt")
            cache_dir = cache_root / "v1.2.3" / "linux_amd64"
            cache_dir.mkdir(parents=True)
            exe = cache_dir / "irasutoya"
            exe.write_text("verified", encoding="utf-8")
            with mock.patch.object(subject, "BIN_DIR", cache_root), mock.patch.object(subject, "latest_release", return_value=release), mock.patch.object(
                subject, "download_bytes", side_effect=AssertionError("download should not run")
            ):
                write_valid_marker(cache_dir, exe, target)
                self.assertEqual(subject.download_release_binary(subject.DEFAULT_REPO, target), exe.resolve())

    def test_download_release_binary_rejects_invalid_verified_cache(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cache_root = Path(tmp) / "bin"
            target = subject.Target("linux", "amd64", ".tar.gz", "irasutoya")
            release = release_with("irasutoya_v1.2.3_linux_amd64.tar.gz", "checksums.txt")
            cache_dir = cache_root / "v1.2.3" / "linux_amd64"
            cache_dir.mkdir(parents=True)
            exe = cache_dir / "irasutoya"
            exe.write_text("unverified", encoding="utf-8")
            (cache_dir / ".verified.json").write_text("{}", encoding="utf-8")
            with mock.patch.object(subject, "BIN_DIR", cache_root), mock.patch.object(subject, "latest_release", return_value=release), mock.patch.object(
                subject, "download_bytes", side_effect=subject.IrasutoyaSearchError("download attempted")
            ):
                with self.assertRaisesRegex(subject.IrasutoyaSearchError, "download attempted"):
                    subject.download_release_binary(subject.DEFAULT_REPO, target)


class ParserAndCommandTests(unittest.TestCase):
    def test_parse_multiple_result_blocks(self) -> None:
        output = """Page URL:    https://example.test/a
Title:       Cat
Description: Cat desc
Image URL:   https://example.test/a.png

Page URL:    https://example.test/b
Title:       Dog
Description: Dog desc
Image URL:   https://example.test/b.png
"""
        results = subject.parse_results(output)
        self.assertEqual(len(results), 2)
        self.assertEqual(results[0]["title"], "Cat")
        self.assertEqual(results[1]["image_urls"], ["https://example.test/b.png"])

    def test_parse_adjacent_result_blocks_without_blank_separator(self) -> None:
        output = """Page URL:    https://example.test/a
Title:       Cat
Description: Cat desc
Image URL:   https://example.test/a.png
Page URL:    https://example.test/b
Title:       Dog
Description: Dog desc
Image URL:   https://example.test/b.png
"""
        results = subject.parse_results(output)
        self.assertEqual(len(results), 2)
        self.assertEqual(results[0]["page_url"], "https://example.test/a")
        self.assertEqual(results[1]["title"], "Dog")

    def test_open_images_flag_is_explicit_only(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            exe = Path(tmp) / "irasutoya"
            exe.write_text("", encoding="utf-8")
            completed = subject.subprocess.CompletedProcess(
                args=[],
                returncode=0,
                stdout="Page URL:    u\nTitle:       t\nDescription: d\nImage URL:   i\n",
                stderr="",
            )
            with mock.patch.object(subject.subprocess, "run", return_value=completed) as run:
                subject.run_search(exe, "cat", False)
                self.assertNotIn("--open-images", run.call_args.args[0])
                self.assertEqual(run.call_args.kwargs["encoding"], "utf-8")
                self.assertEqual(run.call_args.kwargs["errors"], "replace")
                subject.run_search(exe, "cat", True)
                self.assertIn("--open-images", run.call_args.args[0])

    def test_summary_limits_image_urls_by_default(self) -> None:
        summary = subject.summarize(
            Path("irasutoya"),
            "cat",
            [{"title": "Cat", "image_urls": ["1", "2", "3", "4", "5", "6"]}],
            False,
            5,
        )
        self.assertIn("Image URL: 5", summary)
        self.assertNotIn("Image URL: 6", summary)
        self.assertIn("Image URLs omitted: 1", summary)


if __name__ == "__main__":
    unittest.main(verbosity=2)
