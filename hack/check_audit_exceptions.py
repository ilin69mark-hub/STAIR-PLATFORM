#!/usr/bin/env python3
"""Проверка синхрона маркеров AUDIT-EXCEPTION с реестром исключений.

Контракт (см. docs/SECURITY_EXCEPTIONS.yml):
  - каждый маркер AUDIT-EXCEPTION(EXX) в коде обязан иметь парный ID
    в реестре, иначе exit 1 (нельзя тихо захардкодить исключение в коде);
  - записи реестра без живых маркеров и дрейф строк якорей — warnings.
Зависимостей нет (только stdlib): ID реестра вынимаются регекспом
  `^-\\s*id:\\s*(E\\d+)` — схема файла зафиксирована шапкой реестра.
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
REGISTRY = ROOT / "docs" / "SECURITY_EXCEPTIONS.yml"

SKIP_DIRS = {".git", "node_modules", "dist", "__pycache__", ".venv", "venv"}
SKIP_FILES = {"package-lock.json"}

REGISTRY_ID_RE = re.compile(r"^\s*-\s*id:\s*(E\d+)\s*$")
MARKER_RE = re.compile(r"AUDIT-EXCEPTION\((E\d+)\)")
ANCHOR_RE = re.compile(r"^\s*-\s*\{\s*file:\s*(\S+?)\s*,\s*line:\s*(\d+)\s*\}")


def registry_ids() -> set[str]:
    ids = set()
    for line in REGISTRY.read_text(encoding="utf-8").splitlines():
        m = REGISTRY_ID_RE.match(line)
        if m:
            ids.add(m.group(1))
    return ids


def repo_markers() -> dict[str, list[str]]:
    """ID -> список мест 'файл:строка'."""
    found: dict[str, list[str]] = {}
    for path in sorted(ROOT.rglob("*")):
        if not path.is_file() or path.name in SKIP_FILES:
            continue
        try:
            rel = path.relative_to(ROOT)
        except ValueError:
            continue
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        if rel == Path("docs/SECURITY_EXCEPTIONS.yml"):
            continue  # сам реестр (в шапке пример E00)
        try:
            text = path.read_text(encoding="utf-8")
        except (UnicodeDecodeError, OSError):
            continue  # бинарники
        for i, line in enumerate(text.splitlines(), 1):
            for m in MARKER_RE.finditer(line):
                found.setdefault(m.group(1), []).append(f"{rel}:{i}")
    return found


def main() -> int:
    if not REGISTRY.exists():
        print(f"FAIL: registry missing: {REGISTRY}")
        return 1
    reg = registry_ids()
    if not reg:
        print("FAIL: no exception IDs parsed from registry")
        return 1
    markers = repo_markers()

    errors = []
    for mid, places in sorted(markers.items()):
        if mid not in reg:
            errors.append(
                f"orphan marker {mid} without registry entry: {', '.join(places)}"
            )
    if errors:
        print("FAIL: AUDIT-EXCEPTION markers without registry IDs:")
        for e in errors:
            print(f"  - {e}")
        return 1

    # Warnings: записи без живых маркеров; якоря, чьих файлов нет.
    warned = False
    for rid in sorted(reg):
        if rid not in markers:
            print(f"WARN: registry entry {rid} has no live markers")
            warned = True
    text = REGISTRY.read_text(encoding="utf-8")
    for m in ANCHOR_RE.finditer(text):
        f, line = m.group(1), m.group(2)
        p = ROOT / f
        if not p.exists():
            print(f"WARN: anchor file missing: {f}:{line}")
            warned = True
            continue
        try:
            content = p.read_text(encoding="utf-8")
        except OSError:
            continue
        if "AUDIT-EXCEPTION(" not in content:
            # Маркер из файла уехал (дрейф/удаление) — сверить вручную.
            print(f"WARN: anchor file has no markers (drift?): {f} (registry says line {line})")
            warned = True
    print(f"OK: {len(markers)} marker IDs paired with registry ({len(reg)} entries)")
    return 0 if not errors else 1


if __name__ == "__main__":
    sys.exit(main())
