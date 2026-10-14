# -*- coding: utf-8 -*-
"""Генератор parts-manifest.json из эталона FreeCAD (S-198).

Что делает: открывает эталонную модель, для каждой детали из
models/parts.yaml находит её объекты, снимает габариты и объёмы
(включая субтела: наконечник TIP-001 — это 7 тел) и пишет
models/parts-manifest.json — машиночитаемый манифест «тело → деталь».

Запуск — ТОЛЬКО через freecadcmd, скрипту нужен модуль FreeCAD.
Обязательно с PYTHONUNBUFFERED=1: вывод freecadcmd идёт в pipe
буферизованно, и без этого сообщения теряются при нештатном выходе:

    PYTHONUNBUFFERED=1 freecadcmd hack/gen_part_manifest.py
    STAIR_MANIFEST_CHECK=1 PYTHONUNBUFFERED=1 freecadcmd hack/gen_part_manifest.py

Почему без аргументов командной строки: freecadcmd разбирает их как свои
опции и файлы проектов (флаг --pass здесь не разделитель, а опция со
значением; непереданные опции валят запуск). Поэтому все пути скрипт
выводит из собственного расположения, а режим — из STAIR_MANIFEST_CHECK.

Байт-в-байт: в JSON нет дат и времён, ключи отсортированы, числа округлены.
Иначе режим сверки врал бы на каждой машине и после каждого запуска.
Единственное исключение — freecad_version: это provenance (кем снято),
а не содержимое. В CI стоит другой билд FreeCAD, и строгое сравнение
версий валило бы честную проверку. Поэтому CHECK сравнивает всё, кроме
версии (она лишь обязана быть непустой), а локальный прогон тем же
бинарём сходится и побайтово.

Связь с прошлым: логика base_name и округления повторяет
../freecad-out/export_catalog.py один в один, чтобы выход сверялся с
reference_parts.json яблоки-к-яблокам. Смысл (какой объект — какая деталь)
скрипт НЕ выводит из имён: он берёт source из parts.yaml.
"""

import hashlib
import json
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# Пути переопределяются окружением для второй модели (сид S-203):
# STAIR_PARTS_YAML=models/seed/parts.yaml
# STAIR_MANIFEST_JSON=models/seed/seed-manifest.json
# Дефолты — эталонная модель. Env, а не аргументы: см. шапку про freecadcmd.
PARTS_YAML = os.environ.get("STAIR_PARTS_YAML",
                            os.path.join(REPO, "models", "parts.yaml"))
MANIFEST_JSON = os.environ.get("STAIR_MANIFEST_JSON",
                               os.path.join(REPO, "models", "parts-manifest.json"))
CHECK = os.environ.get("STAIR_MANIFEST_CHECK") == "1"

import yaml  # noqa: E402  (комплектный python FreeCAD его имеет — проверено)
import FreeCAD as App  # noqa: E402


def base_name(n):
    """Семейство объекта: Rail_Cap_Start/End -> Rail_Cap, Web_A_01 -> Web.

    Копия base_name из ../freecad-out/export_catalog.py. Здесь она нужна
    только как группировка: какие объекты относятся к одному source из
    parts.yaml. Смысл привязки задаёт YAML, а не эта функция.
    """
    if n.startswith("Rail_Cap"):
        return "Rail_Cap"
    if n.startswith("Rail_Rigel"):
        return "Rail_Rigel_1"
    for pre in ("Web", "Heel", "Anchor", "FirstWeb", "TopAnchor"):
        if n == pre or n.startswith(pre + "_"):
            return pre
    if n == "Rail_Portun":
        return "Rail"
    for pre in ("Tread", "Riser", "Stringer", "Rail_Post", "Rail_Tip",
                "Rail_Lid", "Rail_Clamp", "Rail_Deco", "Rail_Slip"):
        if n == pre or n.startswith(pre + "_") or \
                re.match(re.escape(pre) + r"\d+$", n):
            return pre
    return n


def bbox_of(shape):
    bb = shape.BoundBox
    return {"l": round(bb.XLength, 2),
            "w": round(bb.YLength, 2),
            "h": round(bb.ZLength, 2)}


def axis_len_of(shape):
    """Длина трубы по оси: самая длинная прямая кромка тела.

    У цилиндра это шов — ровно L. Круги (торцы, длина πD) и дуги
    отбрасываем: они не ось. Нужно, чтобы каталог сверялся с осью,
    а не с проекцией bbox (S-206: проекция короче оси на ~20 %).
    Нет прямых кромок — 0.0 (гейт такую деталь отбракует, а не скроет).
    """
    best = 0.0
    try:
        edges = shape.Edges
    except (AttributeError, Exception):
        return 0.0
    for e in edges:
        try:
            if e.Curve.TypeId == "Part::GeomLine" and e.Length > best:
                best = e.Length
        except (AttributeError, Exception):
            continue
    return round(best, 2)


def build_manifest():
    with open(PARTS_YAML, encoding="utf-8") as f:
        spec = yaml.safe_load(f)
    model_rel = spec["source_model"]
    model_path = os.path.join(REPO, "models", model_rel)
    with open(model_path, "rb") as f:
        digest = hashlib.sha256(f.read()).hexdigest()

    doc = App.openDocument(model_path)
    try:
        # Группировка объектов по семействам — в порядке документа
        # (детерминировано для данного файла; представитель — первый).
        families = {}
        for o in doc.Objects:
            if not hasattr(o, "Shape"):
                continue
            families.setdefault(base_name(o.Name), []).append(o)

        parts = []
        for entry in spec["parts"]:
            key = base_name(entry["source"])
            objs = families.get(key, [])
            if not objs:
                raise SystemExit(
                    "MANIFEST FAIL: деталь %s (source %s): в модели нет "
                    "ни одного объекта семейства %r" %
                    (entry["id"], entry["source"], key))
            rep = objs[0]
            # Shape трогаем один раз на объект: повторные обращения к
            # o.Shape под freecadcmd роняли процесс. Всё ниже — из кэша.
            shapes = [o.Shape for o in objs]
            rep_shape = shapes[0]
            solids = []
            for o, sh in zip(objs, shapes):
                sub = []
                try:
                    subshapes = sh.Solids
                except AttributeError:
                    subshapes = []
                for s in subshapes:
                    sub.append({"bbox": bbox_of(s),
                                "volume": round(s.Volume, 1)})
                solids.append({"object": o.Name,
                               "bbox": bbox_of(sh),
                               "volume": round(sh.Volume, 1),
                               "subsolids": sub})
            rec = {"id": entry["id"],
                   "name": entry.get("name", ""),
                   "role": entry.get("role", ""),
                   "group": entry.get("group", ""),
                   "form": entry.get("form", ""),
                   "material": entry.get("material", ""),
                   "source_object": rep.Name,
                   # count — ЗАМЕРЕННЫЕ объекты; instances — заявленные
                   # изделия из YAML. Совпадают, пока деталь = одно тело
                   # (весь эталон S-198). Расходятся у составных фитингов
                   # сида (TIP: 10 тел на 2 изделия): заказывать и считать
                   # надо по instances, а count — для контроля полноты.
                    "count": len(objs),
                    "instances": entry.get("count", len(objs)),
                    "bbox": bbox_of(rep_shape),
                    "volume": round(rep_shape.Volume, 1),
                    "solids": solids}
            tube = (spec.get("tubes") or {}).get(entry["id"])
            if tube:
                rec["tube"] = {"od": tube["od"], "wall": tube["wall"]}
                # Замер оси прямо в модели (S-206): каталог обязан
                # совпадать с ним, а не с проекцией bbox.
                rec["axis_len"] = axis_len_of(rep_shape)
                # Припуск сплавления (POST-001 эталона: шов короче покупной
                # трубы на высоту фланца). Заявлен в YAML, а не зашит в гейт.
                if tube.get("axis_allowance"):
                    rec["axis_allowance"] = tube["axis_allowance"]
            if entry.get("note"):
                rec["note"] = entry["note"]
            parts.append(rec)

        unmatched = sorted(
            o.Name for fam, objs in families.items() for o in objs
            if not any(base_name(e["source"]) == fam
                       for e in spec["parts"]))
        if unmatched:
            print("MANIFEST WARN: объекты без детали в parts.yaml: %s" %
                  ", ".join(unmatched))

        return {"generated_by": "hack/gen_part_manifest.py (S-198)",
                "freecad_version": list(App.Version()),
                "source_model": model_rel,
                "source_sha256": digest,
                "tolerance_mm": 0.3,
                "tolerance_volume_pct": 3.0,
                "parts": parts}
    finally:
        App.closeDocument(doc.Name)


def canonical(manifest):
    """Канонический текст для сравнения: всё, кроме версии FreeCAD.

    Версия маскируется (см. шапку: provenance, а не содержимое), но
    обязана присутствовать: манифест без указания, кем он снят, —
    это данные без происхождения, такому веры нет.
    """
    if not manifest.get("freecad_version"):
        raise SystemExit("MANIFEST FAIL: не указана версия FreeCAD")
    masked = dict(manifest)
    masked["freecad_version"] = []
    return json.dumps(masked, indent=2, ensure_ascii=False,
                      sort_keys=True) + "\n"


def main():
    manifest = build_manifest()
    text = json.dumps(manifest, indent=2, ensure_ascii=False,
                      sort_keys=True) + "\n"
    if CHECK:
        try:
            with open(MANIFEST_JSON, encoding="utf-8") as f:
                committed = json.load(f)
        except OSError:
            print("MANIFEST FAIL: %s отсутствует — сначала запишите "
                  "его обычным запуском" % MANIFEST_JSON)
            raise SystemExit(1)
        if canonical(committed) != canonical(manifest):
            print("MANIFEST FAIL: манифест разошёлся с моделью — "
                  "перегенерируйте его обычным запуском и проверьте дифф")
            raise SystemExit(1)
        print("MANIFEST OK: %s свежий (%d деталей)" %
              (MANIFEST_JSON, len(manifest["parts"])))
    else:
        with open(MANIFEST_JSON, "w", encoding="utf-8") as f:
            f.write(text)
        print("MANIFEST WROTE: %s (%d деталей)" %
              (MANIFEST_JSON, len(manifest["parts"])))
    return 0


main()
