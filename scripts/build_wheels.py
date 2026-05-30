#!/usr/bin/env python3
"""Build platform-specific wheels that embed the colref binary."""

import hashlib
import re
import sys
import tarfile
import zipfile
from base64 import urlsafe_b64encode
from pathlib import Path

# (goos, goarch) -> (wheel_platform_tag, bin_name)
PLATFORMS = [
    ("linux",   "amd64",  "manylinux_2_34_x86_64", "colref"),
    ("linux",   "arm64",  "manylinux_2_34_aarch64", "colref"),
    ("darwin",  "amd64",  "macosx_10_9_x86_64",     "colref"),
    ("darwin",  "arm64",  "macosx_11_0_arm64",       "colref"),
    ("windows", "amd64",  "win_amd64",               "colref.exe"),
]

_INIT_PY = '"""colref — check column references before deleting."""\n'

_MAIN_PY = """\
import os
import subprocess
import sys


def main():
    here = os.path.dirname(os.path.abspath(__file__))
    bin_name = "colref.exe" if sys.platform == "win32" else "colref"
    binary = os.path.join(here, "_bin", bin_name)
    try:
        os.chmod(binary, 0o755)
    except OSError:
        pass
    if sys.platform == "win32":
        sys.exit(subprocess.call([binary] + sys.argv[1:]))
    else:
        os.execvp(binary, [binary] + sys.argv[1:])


if __name__ == "__main__":
    main()
"""


def _normalize_version(version: str) -> str:
    """Translate Go-style tag to PEP 440 (strip leading v, normalize pre-release)."""
    version = re.sub(r"^v", "", version)
    version = re.sub(r"-rc(\d+)$",    r"rc\1",     version)
    version = re.sub(r"-beta(\d+)$",  r"b\1",      version)
    version = re.sub(r"-alpha(\d+)$", r"a\1",      version)
    version = re.sub(r"-post(\d+)$",  r".post\1",  version)
    version = re.sub(r"-dev(\d+)$",   r".dev\1",   version)
    return version


def _record_hash(data: bytes) -> str:
    digest = urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()
    return f"sha256={digest}"


def _build_wheel(
    version: str,
    platform_tag: str,
    bin_name: str,
    binary_data: bytes,
    license_data: bytes,
    readme_data: bytes,
    out_dir: Path,
) -> None:
    wheel_filename = f"colref-{version}-py3-none-{platform_tag}.whl"
    dist_info = f"colref-{version}.dist-info"

    metadata = (
        f"Metadata-Version: 2.3\n"
        f"Name: colref\n"
        f"Version: {version}\n"
        f"Summary: Check whether a database column is still referenced in your codebase before you delete it\n"
        f"Home-page: https://shinagawa-web.github.io/colref/\n"
        f"License: MIT\n"
        f"Requires-Python: >=3.8\n"
        f"Classifier: Development Status :: 4 - Beta\n"
        f"Classifier: Environment :: Console\n"
        f"Classifier: Intended Audience :: Developers\n"
        f"Classifier: License :: OSI Approved :: MIT License\n"
        f"Classifier: Programming Language :: Python :: 3\n"
        f"Classifier: Topic :: Software Development :: Quality Assurance\n"
        f"Keywords: django,rails,database,linter\n"
        f"Project-URL: Source, https://github.com/shinagawa-web/colref\n"
        f"Project-URL: Bug Tracker, https://github.com/shinagawa-web/colref/issues\n"
        f"Description-Content-Type: text/markdown\n"
        f"\n"
        + readme_data.decode()
    )

    wheel_meta = (
        f"Wheel-Version: 1.0\n"
        f"Generator: colref-release\n"
        f"Root-Is-Purelib: false\n"
        f"Tag: py3-none-{platform_tag}\n"
    )

    entry_points = "[console_scripts]\ncolref = colref.__main__:main\n"

    files: list[tuple[str, bytes, bool]] = [
        ("colref/__init__.py",                        _INIT_PY.encode(),      False),
        ("colref/__main__.py",                        _MAIN_PY.encode(),      False),
        (f"colref/_bin/{bin_name}",                   binary_data,            True),
        (f"{dist_info}/licenses/LICENSE",             license_data,           False),
        (f"{dist_info}/METADATA",                     metadata.encode(),      False),
        (f"{dist_info}/WHEEL",                        wheel_meta.encode(),    False),
        (f"{dist_info}/entry_points.txt",             entry_points.encode(),  False),
    ]

    records = [f"{path},{_record_hash(data)},{len(data)}" for path, data, _ in files]
    records.append(f"{dist_info}/RECORD,,")
    record_data = "\n".join(records) + "\n"

    with zipfile.ZipFile(out_dir / wheel_filename, "w", zipfile.ZIP_DEFLATED) as whl:
        for arc_path, data, executable in files:
            info = zipfile.ZipInfo(arc_path)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = (0o755 if executable else 0o644) << 16
            whl.writestr(info, data)
        whl.writestr(f"{dist_info}/RECORD", record_data)

    print(f"  built {wheel_filename}")


def main() -> None:
    if len(sys.argv) != 3:
        sys.exit(f"Usage: {sys.argv[0]} <version> <artifacts_dir>")

    raw_version = sys.argv[1]
    version = _normalize_version(raw_version)
    artifacts_dir = Path(sys.argv[2])
    out_dir = Path("dist")
    out_dir.mkdir(exist_ok=True)

    repo_root = Path(__file__).parent.parent
    license_data = (repo_root / "LICENSE").read_bytes()
    readme_data = (repo_root / "README.md").read_bytes()

    for goos, goarch, platform_tag, bin_name in PLATFORMS:
        tarball = artifacts_dir / f"colref_{raw_version}_{goos}_{goarch}.tar.gz"
        if not tarball.exists():
            print(f"  skip {goos}/{goarch}: {tarball.name} not found")
            continue

        with tarfile.open(tarball, "r:gz") as tf:
            member = tf.getmember(bin_name)
            f = tf.extractfile(member)
            if f is None:
                raise RuntimeError(f"{bin_name} is not a regular file in {tarball.name}")
            binary_data = f.read()

        _build_wheel(version, platform_tag, bin_name, binary_data, license_data, readme_data, out_dir)


if __name__ == "__main__":
    main()
