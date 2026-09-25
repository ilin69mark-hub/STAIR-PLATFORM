#!/usr/bin/env python3
"""Загрузка PBR-наборов (CC0) для 3D-вьювера — этап 1 «студийный 3D».

Источник: ambientCG (CC0 1.0, commercial OK, атрибуция не требуется).
Скрипт воспроизводим: фиксирует источник, лицензию и SHA-1 каждого файла
в assets/pbr/SOURCES.md, поэтому набор можно пересобрать в любой момент.

Использование:
    python3 hack/fetch_pbr.py                 # скачать недостающие
    python3 hack/fetch_pbr.py --list          # показать маппинг
    python3 hack/fetch_pbr.py --only oak      # один материал

Из архива 1K-JPG оставляются только Color/Normal/Roughness(+AO) и
приводятся к 1024px (PIL). Остальные карты (Displacement, AO отдельный
файл и т.п.) не нужны вьюверу и удаляются.
"""
from __future__ import annotations

import argparse
import hashlib
import io
import sys
import urllib.request
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DEST = ROOT / "assets" / "pbr"
SOURCES = DEST / "SOURCES.md"
RES = 1024

# material code -> (ambientCG asset id, человекочитаемое имя)
MATERIALS: dict[str, tuple[str, str]] = {
    "WOOD-OAK": ("Wood095", "Дуб светлый, полированный"),
    "WOOD-WALNUT": ("Wood094", "Орех американский, тёмный"),
    "WOOD-ASH": ("Wood092", "Ясень, светлый"),
    "WOOD-SOFT": ("Wood049", "Хвойные доски, мягкий рисунок"),
    "STEEL-S235": ("Metal032", "Сталь окрашенная, гладкая"),
    "STEEL-CORTEN": ("Metal022", "Кортен / weathering steel"),
    "ALUM-5083": ("Metal050A", "Алюминий шлифованный"),
}

KEEP_MAPS = ("color", "normal", "roughness", "ambientocclusion")
URL = "https://ambientcg.com/get?file={asset}_1K-JPG.zip"
UA = "stair-platform-pbr-fetch/1.0 (+CC0 asset fetch)"


def sha1(data: bytes) -> str:
    return hashlib.sha1(data).hexdigest()


def fetch(url: str) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": UA})
    with urllib.request.urlopen(req, timeout=120) as resp:  # noqa: S310 (фикс. хост)
        return resp.read()


def resize(data: bytes, is_normal: bool) -> bytes:
    """Приводит карту к RES×RES JPEG. Нормали остаются в sRGB-данных, но
    three.js трактует normalMap как линейную карту (colorSpace = NoColorSpace),
    поэтому JPEG без потери здесь допустим."""
    from PIL import Image  # локальный импорт: скрипт не тянет PIL в приложение

    img = Image.open(io.BytesIO(data))
    if img.mode not in ("RGB", "L"):
        img = img.convert("RGB")
    if img.size != (RES, RES):
        img = img.resize((RES, RES), Image.LANCZOS)
    out = io.BytesIO()
    if is_normal:
        img.save(out, format="JPEG", quality=92)
    else:
        img.save(out, format="JPEG", quality=88, optimize=True)
    return out.getvalue()


def save_map(raw: bytes, target: Path, is_normal: bool) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    data = resize(raw, is_normal)
    target.write_bytes(data)
    _write_source_line(target, data)


def _write_source_line(target: Path, data: bytes) -> None:
    rel = target.relative_to(ROOT)
    with SOURCES.open("a", encoding="utf-8") as f:
        f.write(f"| `{rel}` | ambientCG (CC0 1.0) | {len(data) // 1024} KB | `{sha1(data)}` |\n")


def ensure_sources_header() -> None:
    if SOURCES.exists():
        return
    DEST.mkdir(parents=True, exist_ok=True)
    SOURCES.write_text(
        "# PBR-ассеты 3D (CC0)\n\n"
        "Источник: [ambientCG](https://ambientcg.com/) — CC0 1.0 Universal "
        "(коммерческое использование разрешено, атрибуция не требуется).\n"
        "Собрано скриптом `hack/fetch_pbr.py`; карты приведены к 1024px JPEG.\n"
        "HDRI: [Poly Haven](https://polyhaven.com/) — CC0.\n\n"
        "| Файл | Источник | Размер | SHA-1 |\n|---|---|---|---|\n",
        encoding="utf-8",
    )


def fetch_material(code: str) -> bool:
    asset, human = MATERIALS[code]
    out_dir = DEST / code
    have = sorted(out_dir.glob("*.jpg")) if out_dir.exists() else []
    if len(have) >= 2:
        print(f"  {code}: уже есть {len(have)} карт — пропускаю")
        return True
    print(f"  {code} → ambientCG/{asset} ({human})")
    try:
        blob = fetch(URL.format(asset=asset))
    except Exception as exc:  # noqa: BLE001
        print(f"    ОШИБКА загрузки: {exc}", file=sys.stderr)
        return False

    saved = 0
    with zipfile.ZipFile(io.BytesIO(blob)) as zf:
        for name in zf.namelist():
            lname = name.lower()
            if not lname.endswith((".jpg", ".jpeg")):
                continue
            if not any(f"_{mapname}" in lname for mapname in KEEP_MAPS):
                continue
            if "disp" in lname or "preview" in lname or "thumb" in lname:
                continue
            try:
                raw = zf.read(name)
            except KeyError:
                continue
            stem = Path(name).stem.lower()
            if "_normalgl" in stem or "_normaldx" in stem or "_normal" in stem:
                target, normal = out_dir / "normal.jpg", "_normalgl" in stem
            elif "_roughness" in stem:
                target, normal = out_dir / "roughness.jpg", False
            elif "_color" in stem or "_albedo" in stem or "_diffuse" in stem:
                target, normal = out_dir / "color.jpg", False
            elif "_ao" in stem or "_ambientocclusion" in stem:
                target, normal = out_dir / "ao.jpg", False
            else:
                continue
            save_map(raw, target, normal)
            saved += 1
    if saved == 0:
        print("    в архиве не нашлось PBR-карт", file=sys.stderr)
        return False
    print(f"    сохранено карт: {saved} → {out_dir.relative_to(ROOT)}")
    return True


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--list", action="store_true", help="показать маппинг и выход")
    ap.add_argument("--only", help="скачать один материал по коду")
    args = ap.parse_args()

    if args.list:
        for code, (asset, human) in MATERIALS.items():
            print(f"{code:14} {asset:16} {human}")
        return 0

    ensure_sources_header()
    codes = [args.only] if args.only else list(MATERIALS)
    failed: list[str] = []
    for code in codes:
        if code not in MATERIALS:
            print(f"неизвестный код: {code}", file=sys.stderr)
            return 2
        if not fetch_material(code):
            failed.append(code)

    if failed:
        print(f"\nне удалось: {', '.join(failed)}", file=sys.stderr)
        return 1
    print(f"\nготово. Манифест: {SOURCES.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
