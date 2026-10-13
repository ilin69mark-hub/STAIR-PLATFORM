# -*- coding: utf-8 -*-
"""Авторская модель минимального марша — близнец сида (S-203).

Что это: прямой марш 900×1080, 6 ступеней (подъём 180, проступь 270),
косоур-гребёнка 8 мм, проступь 40, подступенки, перила 900 слева —
та же конфигурация, что seedConfig() в engine/geometry/seed_test.go.
Каждая деталь — отдельный объект Part::Feature (требование владельца).

Запуск — ТОЛЬКО через freecadcmd (нужен модуль FreeCAD):
    PYTHONUNBUFFERED=1 freecadcmd hack/build_seed_model.py

Номиналы — из чертежа узла (comb_frame.go) и билдеров движка, а НЕ из
вывода движка: иначе сверка «движок против модели» сравнивала бы движок
с самим собой. Совпадение в пределах допусков означает, что чертёж
прочитан дважды одинаково, — это и есть проверка.

Кадр ЛОКАЛЬНЫЙ (x от 0, без сдвига approachShift): модель — библиотека
деталей, сдвиг даёт калькулятор при размещении (этап 2).

Осознанные упрощения (влияют на объём заведомо меньше допуска 3 %):
- фаска носа R10 пропущена (+0,2 % к проступям);
- поручень — прямой короб 50×40 без скругления кромок;
- ориентация 3 отверстий Ø9 в диске цанги произвольна (90/210/330°).

Осознанное повторение поведения движка 1:1 — S-208a закрыт в движке,
модель повторяет исправленное: поручень-труба Ø42,4 через верхи стоек
(лежит на наконечниках), стойки 805,7, заглушки в торцах трубы.

В конце скрипт проверяет сам себя: счётчики по префиксам, габарит,
объём МОДЕЛИ против объёма сида движка 109079232 мм³ (±0,5 %).
Перила в объёмную сверку не входят: их правда спорна (S-208).
Не сошлось — исключение и никакого файла.
"""

import math
import os
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(REPO, "models", "seed", "minimal-straight.FCStd")

import FreeCAD as App  # noqa: E402
import Part  # noqa: E402

# --- Номиналы сида (seedConfig + CombFrameDefaults + каталог) ---
N, B, H, ST, W = 6, 270.0, 180.0, 40.0, 900.0
T_STR = 8.0
SPINE = 300.0
NOSING = 20.0
HEEL_T, HEEL_W, INSET, GAP = 4.0, 50.0, 5.0, 5.0
HOLE_D = 10.0
ANCHOR_D = 12.0
SLOT_L = 30.0
RH = 900.0
BALUSTER_EDGE_INSET = 15.0
POST_OD, POST_WALL = 42.4, 1.5
RIGEL_OD, RIGEL_WALL = 12.0, 1.0

# Объём сида движка (seed_test.go): близнец обязан сойтись в ±3 %.
SEED_VOLUME = 109079232.0

V = App.Vector


def box(doc, name, x0, y0, z0, dx, dy, dz):
    o = doc.addObject("Part::Feature", name)
    o.Shape = Part.makeBox(dx, dy, dz, V(x0, y0, z0))
    return o


def z_tube(doc, name, x, y, z0, h, od, wall=0.0):
    """Вертикальная труба (wall=0 — сплошной цилиндр)."""
    outer = Part.makeCylinder(od / 2, h, V(x, y, z0), V(0, 0, 1))
    if wall > 0:
        inner = Part.makeCylinder(od / 2 - wall, h + 2, V(x, y, z0 - 1), V(0, 0, 1))
        outer = outer.cut(inner)
    o = doc.addObject("Part::Feature", name)
    o.Shape = outer
    return o


def dir_tube(doc, name, a, u, length, od, wall=0.0):
    """Труба из точки a вдоль единичного u."""
    zc = Part.makeCylinder(od / 2, length, V(0, 0, 0), V(0, 0, 1))
    if wall > 0:
        zc = zc.cut(Part.makeCylinder(od / 2 - wall, length + 2,
                                      V(0, 0, -1), V(0, 0, 1)))
    rot = App.Rotation(V(0, 0, 1), V(*u))
    o = doc.addObject("Part::Feature", name)
    o.Shape = zc
    o.Placement = App.Placement(V(*a), rot)
    return o


def cut_holes_z(obj, holes):
    """Вычесть вертикальные цилиндры [(x, y, r, z0, z1)] из объекта."""
    sh = obj.Shape
    for (x, y, r, z0, z1) in holes:
        tool = Part.makeCylinder(r, z1 - z0, V(x, y, z0), V(0, 0, 1))
        sh = sh.cut(tool)
    obj.Shape = sh


def cut_holes_x(obj, holes):
    """Вычесть цилиндры вдоль X [(y, z, r, x0, x1)]."""
    sh = obj.Shape
    for (y, z, r, x0, x1) in holes:
        tool = Part.makeCylinder(r, x1 - x0, V(x0, y, z), V(1, 0, 0))
        sh = sh.cut(tool)
    obj.Shape = sh


def slot_tool_x(x, y, z0, z1, length, dia, along):
    """Стадион-паз 30×12: короб + 2 цилиндра, вдоль X или Y."""
    r = dia / 2
    straight = length - dia
    if along == "x":
        s = Part.makeBox(straight, dia, z1 - z0,
                         V(x - straight / 2, y - r, z0))
        c1 = Part.makeCylinder(r, z1 - z0, V(x - straight / 2, y, z0), V(0, 0, 1))
        c2 = Part.makeCylinder(r, z1 - z0, V(x + straight / 2, y, z0), V(0, 0, 1))
    else:
        s = Part.makeBox(dia, straight, z1 - z0,
                         V(x - r, y - straight / 2, z0))
        c1 = Part.makeCylinder(r, z1 - z0, V(x, y - straight / 2, z0), V(0, 0, 1))
        c2 = Part.makeCylinder(r, z1 - z0, V(x, y + straight / 2, z0), V(0, 0, 1))
    return s.fuse(c1).fuse(c2)


def hole_row(length, d, edge_k=2.0, pitch_k=3.0, stride=2, center=False):
    """Тот же HoleRow, что в движке (comb.go): равномерный ряд."""
    margin = edge_k * d
    step = float(stride) * pitch_k * d
    cnt = int(math.floor((length - 2 * margin) / step)) + 1
    if cnt < 2:
        cnt = 2
    pitch = (length - 2 * margin) / float(cnt - 1)
    xs = [margin + pitch * i for i in range(cnt)]
    if center and len(xs) > 1:
        c = length / 2
        if all(abs(c - x) > d for x in xs):
            xs.append(c)
            xs.sort()
    return xs


def unit(v):
    n = math.sqrt(sum(c * c for c in v))
    return tuple(c / n for c in v)


def trace(msg):
    with open("/tmp/stair-probe/build.trace", "a") as f:
        f.write(msg + "\n")
        f.flush()


def build(doc):
    made = []

    def add(o):
        made.append(o.Name)
        return o

    # --- Косоуры: гребёнка combStringerProfile(6,270,180,40,300) ---
    l = math.hypot(B, H)
    first_z = H - ST
    drop_v = SPINE * l / B
    floor_x = max(0.0, (drop_v - first_z) * B / H)
    prof = [(0.0, 0.0), (0.0, first_z)]
    for k in range(N - 1):
        prof.append(((k + 1) * B, (k + 1) * H - ST))
        prof.append(((k + 1) * B, (k + 2) * H - ST))
    top_x, top_z = N * B, N * H - ST
    prof.append((top_x, top_z))
    prof.append((top_x, (H / B) * (top_x - floor_x)))
    prof.append((floor_x, 0.0))
    wire = Part.makePolygon([V(x, 0.0, z) for (x, z) in prof] + [V(prof[0][0], 0.0, prof[0][1])])
    face = Part.Face(wire)
    for name, y in (("Stringer_A", 0.0), ("Stringer_B", W - T_STR)):
        o = doc.addObject("Part::Feature", name)
        o.Shape = face.extrude(V(0.0, T_STR, 0.0))
        o.Placement.Base = V(0.0, y, 0.0)
        add(o)

    # --- Проступи и подступенки ---
    tread_rear = ST + NOSING  # rtFrame=st=40 + нос 20 (Riser=true)
    tread_depth = B + NOSING
    for k in range(N):
        x0 = k * B - tread_rear
        x1 = x0 + tread_depth
        if k == N - 1:
            x1 = N * B  # TopStepExtend: последняя доходит до конца косоура
        zt = (k + 1) * H
        add(box(doc, "Tread_%02d" % (k + 1), x0, 0.0, zt - ST, x1 - x0, W, ST))
        rx0 = k * B - ST
        rz0 = max(k * H - ST, 0.0)
        add(box(doc, "Riser_%02d" % (k + 1), rx0, 0.0, rz0, ST, W, (k + 1) * H - ST - rz0))

    # --- Обойма: пятки по обоим косоурам ---
    for side, hy0 in (("A", T_STR), ("B", W - T_STR - HEEL_W)):
        for k in range(N):
            heel_l = B - 2 * INSET
            x0 = k * B + INSET
            z_top = (k + 1) * H - ST
            o = add(box(doc, "Heel_%s_%02d" % (side, k + 1),
                        x0, hy0, z_top - HEEL_T, heel_l, HEEL_W, HEEL_T))
            xs = hole_row(heel_l, HOLE_D)
            assert len(xs) == 4, xs
            cut_holes_z(o, [(x0 + x, hy0 + HEEL_W / 2, HOLE_D / 2,
                             z_top - HEEL_T - 1, z_top + 1) for x in xs])
        web_h = H - HEEL_T - 2 * GAP
        for k in range(N - 1):
            x_tooth = (k + 1) * B
            z_bot = (k + 1) * H - ST + HEEL_T + GAP
            o = add(box(doc, "Web_%s_%02d" % (side, k + 1),
                        x_tooth, hy0, z_bot, HEEL_T, HEEL_W, web_h))
            zs = hole_row(web_h, HOLE_D, center=True)
            assert len(zs) == 3, zs
            cut_holes_x(o, [(hy0 + HEEL_W / 2, z_bot + z, HOLE_D / 2,
                             x_tooth - 1, x_tooth + HEEL_T + 1) for z in zs])
        o = add(box(doc, "Anchor_%s" % side, 0.0, hy0, 0.0, B, HEEL_W, HEEL_T))
        axs = hole_row(B, ANCHOR_D)
        assert len(axs) == 4, axs
        sh = o.Shape
        tools = []
        tools.append(slot_tool_x(axs[0], hy0 + HEEL_W / 2, -1.0, HEEL_T + 1.0, SLOT_L, ANCHOR_D, "x"))
        tools.append(slot_tool_x(axs[-1], hy0 + HEEL_W / 2, -1.0, HEEL_T + 1.0, SLOT_L, ANCHOR_D, "y"))
        for x in axs[1:-1]:
            tools.append(Part.makeCylinder(ANCHOR_D / 2, HEEL_T + 2,
                                           V(x, hy0 + HEEL_W / 2, -1.0), V(0, 0, 1)))
        for t in tools:
            sh = sh.cut(t)
        o.Shape = sh
        top_z = N * H - ST
        z_top = top_z - (HEEL_T + GAP)
        o = add(box(doc, "TopAnchor_%s" % side, N * B - HEEL_T, hy0,
                    z_top - web_h, HEEL_T, HEEL_W, web_h))
        tzs = hole_row(web_h, ANCHOR_D, center=True)
        assert len(tzs) == 3, tzs
        sh = o.Shape
        yc = hy0 + HEEL_W / 2
        x0 = N * B - HEEL_T
        zt = z_top - web_h
        s1 = Part.makeBox(HEEL_T + 2, ANCHOR_D, SLOT_L,
                          V(x0 - 1, yc - ANCHOR_D / 2, zt + tzs[0] - SLOT_L / 2))
        c1a = Part.makeCylinder(ANCHOR_D / 2, HEEL_T + 2,
                                V(x0 - 1, yc, zt + tzs[0] - SLOT_L / 2 + ANCHOR_D / 2), V(1, 0, 0))
        c1b = Part.makeCylinder(ANCHOR_D / 2, HEEL_T + 2,
                                V(x0 - 1, yc, zt + tzs[0] + SLOT_L / 2 - ANCHOR_D / 2), V(1, 0, 0))
        sh = sh.cut(s1.fuse(c1a).fuse(c1b))
        sh = sh.cut(Part.makeCylinder(ANCHOR_D / 2, HEEL_T + 2,
                                      V(x0 - 1, yc, zt + tzs[1]), V(1, 0, 0)))
        s3 = Part.makeBox(HEEL_T + 2, SLOT_L, ANCHOR_D,
                          V(x0 - 1, yc - SLOT_L / 2, zt + tzs[2] - ANCHOR_D / 2))
        c3a = Part.makeCylinder(ANCHOR_D / 2, SLOT_L,
                                V(x0 - 1, yc - SLOT_L / 2 + ANCHOR_D / 2, zt + tzs[2]), V(0, 1, 0))
        c3b = Part.makeCylinder(ANCHOR_D / 2, SLOT_L,
                                V(x0 - 1, yc + SLOT_L / 2 - ANCHOR_D / 2, zt + tzs[2]), V(0, 1, 0))
        sh = sh.cut(s3.fuse(c3a).fuse(c3b))
        o.Shape = sh
        fw_w = HEEL_W - 2 * INSET
        fw_h = (H - ST) - 2 * HEEL_T - 2 * GAP
        fw_z = HEEL_T + GAP
        o = add(box(doc, "FirstWeb_%s" % side, 0.0, hy0, fw_z, HEEL_T, fw_w, fw_h))
        fzs = hole_row(fw_h, HOLE_D, center=True)
        assert len(fzs) == 3, fzs
        cut_holes_x(o, [(hy0 + fw_w / 2, fw_z + z, HOLE_D / 2, -1.0, HEEL_T + 1.0) for z in fzs])

    # --- Перила (левая сторона): стойки на ступенях 1 и 6 ---
    # Геометрия S-208a: стойки 805,7 (каталог), поручень-труба Ø42,4
    # через верхи стоек (лежит на наконечниках), Y над стойками.
    POST_H = 805.7
    # Вылет центра поручня над верхом стойки: стек площадки (3+27,7−1,3+
    # 19,4−1,6+1,0 = 48,2) + радиус трубы / cos уклона. Без поправки труба
    # врезается в наклонную площадку на ~3,6 мм (та же формула, что
    # railLiftAbovePostTop в движке).
    LIFT = 48.2 + (POST_OD / 2) * math.hypot(B, H) / B  # = 73,7
    bal_y = BALUSTER_EDGE_INSET + 44.0  # edgeInset(0, w, 15+capOD/2)
    slope = H / B
    ang = -math.degrees(math.atan2(N * H, N * B))  # наклон марша: поручень и площадки
    x1 = (1 - 0.5) * B - NOSING
    z1 = H + POST_H + LIFT
    rail_a = (0.0, bal_y, z1 + slope * (0.0 - x1))
    rail_b = (N * B, bal_y, z1 + slope * (N * B - x1))
    seg = (rail_b[0] - rail_a[0], 0.0, rail_b[2] - rail_a[2])
    rail_len = math.sqrt(sum(c * c for c in seg))
    rail_u = unit(seg)
    add(dir_tube(doc, "Rail_Portun", rail_a, rail_u, rail_len, POST_OD, POST_WALL))
    for row, frac in (("1", 0.33), ("2", 0.66)):
        x0, x1 = 115.0, 1465.0
        z0 = H + POST_H * frac
        z1 = 6 * H + POST_H * frac
        seg = (x1 - x0, 0.0, z1 - z0)
        L = math.sqrt(sum(c * c for c in seg))
        add(dir_tube(doc, "Rail_Rigel_%s_01" % row, (x0, bal_y, z0),
                     unit(seg), L, RIGEL_OD, RIGEL_WALL))
    for pi, k in ((1, 1), (2, 6)):
        x = (k - 0.5) * B - NOSING
        z0 = k * H
        z_top = z0 + POST_H
        # Стойка-труба 805,7 (каталог; движок строил 900 — S-208a)
        add(z_tube(doc, "Rail_Post_%02d" % pi, x, bal_y, z0, POST_H, POST_OD, POST_WALL))
        # Держатель: втулка, крышка, шпилька, шарнир, площадка, стакан, крышка стакана
        add(z_tube(doc, "Rail_Tip_%02d_Sleeve" % pi, x, bal_y, z_top - 24.0, 24.0, 39.4, 1.5))
        add(z_tube(doc, "Rail_Tip_%02d_Cap" % pi, x, bal_y, z_top, 3.0, 42.4))
        rod_z = z_top + 3.0
        add(z_tube(doc, "Rail_Tip_%02d_Stud" % pi, x, bal_y, rod_z, 27.7, 12.0, 1.0))
        jz = rod_z + 27.7 - 1.3
        add(z_tube(doc, "Rail_Tip_%02d_Joint" % pi, x, bal_y, jz, 19.4, 14.0, 1.5))
        tip_bottom = jz + 19.4 - 1.6
        # Середина низа ровно на tip_bottom (как в движке S-210):
        # никаких добавок «на половину наклона» — slab кладёт середину
        # на z_base по построению, добавка задирала площадку на 6,4 мм.
        z_base = tip_bottom
        # Стадион 70×50,8: прямые + две полуокружности r=25,4
        # с центрами ±(70−50,8)/2 = ±9,6. Площадка параллельна маршу
        # (тот же угол, что поручень): лежит ПОД трубой (S-208a).
        pts = []
        for cx, a0, a1 in ((9.6, -90.0, 90.0), (-9.6, 90.0, 270.0)):
            for i in range(11):
                a = math.radians(a0 + (a1 - a0) * i / 10)
                pts.append((cx + 25.4 * math.cos(a), 25.4 * math.sin(a)))
        wire = Part.makePolygon([V(px, py, 0.0) for (px, py) in pts] + [V(pts[0][0], pts[0][1], 0.0)])
        o = doc.addObject("Part::Feature", "Rail_Tip_%02d_Plate" % pi)
        o.Shape = Part.Face(wire).extrude(V(0.0, 0.0, 1.0))
        # Контур центрирован (x,y ∈ ±35/±25,4): база = центр, а НЕ минус
        # половина. Было bal_y−25,4 — площадка висела сбоку на полширины
        # (S-210a, глаз владельца).
        o.Placement = App.Placement(V(x, bal_y, z_base), App.Rotation(V(0, 1, 0), ang))
        add(o)
        add(z_tube(doc, "Rail_Deco_%02d_Body" % pi, x, bal_y, z0, 16.0, 88.0, 0.9))
        add(z_tube(doc, "Rail_Deco_%02d_Lid" % pi, x, bal_y, z0 + 16.0, 1.0, 88.0))
        # Цанга: диск Ø86×8 с 3 отверстиями Ø9 на R32 + муфта Ø43×34
        o = add(z_tube(doc, "Rail_Clamp_%02d_Disc" % pi, x, bal_y, z0, 8.0, 86.0))
        holes = []
        for deg in (90.0, 210.0, 330.0):
            a = math.radians(deg)
            holes.append((x + 32.0 * math.cos(a), bal_y + 32.0 * math.sin(a),
                          4.5, z0 - 1.0, z0 + 9.0))
        cut_holes_z(o, holes)
        add(z_tube(doc, "Rail_Slip_%02d_Muff" % pi, x, bal_y, z0 + 8.0, 34.0, 43.0, 0.2))
    # Заглушки торцов поручня: стаканы Ø39,2 на 18 мм внутрь.
    # Конец строится от точки B−u·18 вдоль +u (а не из B вдоль −u):
    # Rotation(Z, −u) разворачивает трубу не туда.
    add(dir_tube(doc, "Rail_Cap_Start", rail_a, rail_u, 18.0, 39.2, 1.5))
    b_in = (rail_b[0] - rail_u[0] * 18.0, rail_b[1] - rail_u[1] * 18.0,
            rail_b[2] - rail_u[2] * 18.0)
    add(dir_tube(doc, "Rail_Cap_End", b_in, rail_u, 18.0, 39.2, 1.5))
    # Глухие донышки 1 мм на всю ширину трубы (S-211): наружная грань
    # на 0,5 мм внутри торца (иначе копланарность с кольцом среза).
    la = (rail_a[0] + rail_u[0] * 0.5, rail_a[1] + rail_u[1] * 0.5,
          rail_a[2] + rail_u[2] * 0.5)
    add(dir_tube(doc, "Rail_Cap_Start_Lid", la, rail_u, 1.0, 42.4))
    lb = (rail_b[0] - rail_u[0] * 1.5, rail_b[1] - rail_u[1] * 1.5,
          rail_b[2] - rail_u[2] * 1.5)
    add(dir_tube(doc, "Rail_Cap_End_Lid", lb, rail_u, 1.0, 42.4))

    return made


def main():
    with open("/tmp/stair-probe/build.trace", "w") as f:
        f.write("start\n")
    doc = App.newDocument("SeedStair")
    trace("doc ok")
    made = build(doc)
    trace("built %d" % len(made))
    doc.recompute()
    trace("recomputed")
    # --- Самопроверка: счётчики, габарит, объём ---
    want_counts = {"Stringer": 2, "Tread": 6, "Riser": 6, "Heel": 12,
                   "Web": 10, "Anchor": 2, "TopAnchor": 2, "FirstWeb": 2,
                   "Rail_Post": 2, "Rail_Portun": 1, "Rail_Rigel": 2,
                   "Rail_Tip": 10, "Rail_Deco": 4, "Rail_Clamp": 2,
                   "Rail_Slip": 2, "Rail_Cap": 4}
    got_counts = {}
    for o in doc.Objects:
        for pre in want_counts:
            if o.Name == pre or o.Name.startswith(pre + "_"):
                got_counts[pre] = got_counts.get(pre, 0) + 1
                break
    for pre, want in sorted(want_counts.items()):
        got = got_counts.get(pre, 0)
        if got != want:
            raise SystemExit("SEED FAIL: %s: объектов %d, ожидалось %d" % (pre, got, want))
    trace("counts ok")
    # Соосность узла: площадка, стойка и поручень на одной вертикали Y=59,
    # площадка по X над своей стойкой (S-210a: было смещение на полширины).
    for o in doc.Objects:
        if o.Name.startswith("Rail_Tip_") and o.Name.endswith("_Plate"):
            bb = o.Shape.BoundBox
            cx, cy = (bb.XMin + bb.XMax) / 2, (bb.YMin + bb.YMax) / 2
            if abs(cy - 59.0) > 0.5:
                raise SystemExit("SEED FAIL: %s cy=%.1f, ось 59" % (o.Name, cy))
            if min(abs(cx - 115.0), abs(cx - 1465.0)) > 1.0:
                raise SystemExit("SEED FAIL: %s cx=%.1f, стойки 115/1465" % (o.Name, cx))
    # Один проход по телам: забираем числа (bbox, объём) в обычные float
    # сразу. Повторные обращения к o.Shape падают (OCC-причуда freecadcmd:
    # второй обход doc.Objects молча роняет процесс), поэтому всё нужное
    # снимается за один проход, а проверки идут уже по числам.
    boxes = []
    vols = []
    for o in doc.Objects:
        trace("measure %s" % o.Name)
        bb = o.Shape.BoundBox
        boxes.append((bb.XMin, bb.XMax, bb.YMin, bb.YMax, bb.ZMin, bb.ZMax))
        vols.append(o.Shape.Volume)
        if bb.ZMax > 2000.0 or bb.XMax > 1625.0:
            trace("TALL %s Z[%.1f, %.1f] X[%.1f, %.1f]" % (
                o.Name, bb.ZMin, bb.ZMax, bb.XMin, bb.XMax))
    trace("measured %d" % len(boxes))
    x0 = min(b[0] for b in boxes)
    x1 = max(b[1] for b in boxes)
    y0 = min(b[2] for b in boxes)
    y1 = max(b[3] for b in boxes)
    z0 = min(b[4] for b in boxes)
    z1 = max(b[5] for b in boxes)
    trace("bbox %.1f %.1f %.1f %.1f %.1f %.1f" % (x0, x1, y0, y1, z0, z1))
    print("SEED bbox: X[%.1f, %.1f] Y[%.1f, %.1f] Z[%.1f, %.1f]" % (x0, x1, y0, y1, z0, z1))
    # XMin −60 (свес первой проступи), XMax ≈1631 (уголок заглушки
    # Rail_Cap_End: труба B−u·18..B плюс Ø/2 поперёк),
    # ZMax ≈2079 (верх поручня-трубы у выхода).
    # Числа — из номиналов выше, а не из файла.
    for label, got, want, tol in (("XMin", x0, -60.0, 0.5), ("XMax", x1, 1631.0, 2.0),
                                  ("YMin", y0, 0.0, 0.5), ("YMax", y1, 900.0, 0.5),
                                  ("ZMin", z0, 0.0, 0.5), ("ZMax", z1, 2079.0, 5.0)):
        if abs(got - want) > tol:
            raise SystemExit("SEED FAIL: габарит %s %.1f, ожидалось %.1f" % (label, got, want))
    total = 0.0
    for v in vols:
        total += v
    trace("total %.1f" % total)
    by_kind = {}
    for o, v in zip(doc.Objects, vols):
        for pre in want_counts:
            if o.Name == pre or o.Name.startswith(pre + "_"):
                by_kind[pre] = by_kind.get(pre, 0.0) + v
                break
    for k in sorted(by_kind):
        trace("SUBTOTAL %s %.1f" % (k, by_kind[k]))
    # Объём МОДЕЛИ (без перил) против сида движка: перила сверяются
    # отдельно, их правда сейчас спорна (S-208), и валить близнеца
    # из-за неё нельзя.
    model_total = sum(v for o, v in zip(doc.Objects, vols)
                      if not o.Name.startswith("Rail_"))
    print("SEED model volume: %.1f (движок %.1f)" % (model_total, SEED_VOLUME))
    if abs(model_total - SEED_VOLUME) / SEED_VOLUME > 0.005:
        raise SystemExit("SEED FAIL: объём модели вне ±0,5 %")
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    doc.saveAs(OUT)
    print("SEED WROTE: %s (%d объектов)" % (OUT, len(made)))


main()
