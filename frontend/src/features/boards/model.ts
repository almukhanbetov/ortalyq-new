// The board document in the editor: items with ids, geometry for selection, moving and resizing, and the view.
import type { BoardItem } from '@/types/api';

/** The initial view at 100% (the same as the server's CanvasWidth × CanvasHeight). */
export const W = 1600;
export const H = 1000;
/** Objects may be placed anywhere inside ±WORLD (the server clamps to the same limit). */
export const WORLD = 20000;

export const PEN_WIDTH = 4;
export const TEXT_SIZE = 40;
export const STICKY_FONT = 24;
export const TEXT_MAX = 500;
export const STICKY_MAX = 1000;

export const ZOOMS = [0.25, 0.33, 0.5, 0.67, 0.75, 0.9, 1, 1.25, 1.5, 2, 3, 4];

/**
 * Ink colours of the drawing and the matching sticky paper. They are part of the saved board (content data), not
 * interface colours, so they are fixed hex values rather than design tokens; the names are for screen readers.
 */
export const INKS = [
  { value: '#1F2937', tint: '#E5E7EB', name: 'black' },
  { value: '#2F80ED', tint: '#D6E8FF', name: 'blue' },
  { value: '#E5484D', tint: '#FFDCDC', name: 'red' },
  { value: '#16A34A', tint: '#D3F5DD', name: 'green' },
  { value: '#F59E0B', tint: '#FFF1A8', name: 'orange' },
  { value: '#9254DE', tint: '#EADCFF', name: 'purple' },
] as const;

export type Item = BoardItem & { id: string };
export type Pt = [number, number];
export interface View { x: number; y: number; z: number }

export const newId = () => Math.random().toString(36).slice(2, 10) + Date.now().toString(36).slice(-4);

/** Boards saved before ids existed: give every item an id (not saved until the next save). */
export function withIds(items: BoardItem[]): Item[] {
  const seen = new Set<string>();
  return items.map((it) => {
    if (it.id && !seen.has(it.id)) { seen.add(it.id); return it as Item; }
    const id = newId();
    seen.add(id);
    return { ...it, id } as Item;
  });
}

const clampW = (v: number) => Math.min(WORLD, Math.max(-WORLD, v));
export const round = (v: number) => Math.round(v * 10) / 10;

export function translate(it: Item, dx: number, dy: number): Item {
  if (it.type === 'stroke' || it.type === 'arrow') {
    return { ...it, points: it.points.map(([x, y]) => [round(clampW(x + dx)), round(clampW(y + dy))] as Pt) };
  }
  return { ...it, x: round(clampW(it.x + dx)), y: round(clampW(it.y + dy)) };
}

export interface Box { x: number; y: number; w: number; h: number }

export function bbox(it: Item): Box {
  if (it.type === 'stroke' || it.type === 'arrow') {
    const xs = it.points.map((p) => p[0]);
    const ys = it.points.map((p) => p[1]);
    const pad = it.width / 2 + (it.type === 'arrow' ? it.width * 3 + 6 : 0);
    const x = Math.min(...xs) - pad;
    const y = Math.min(...ys) - pad;
    return { x, y, w: Math.max(...xs) + pad - x, h: Math.max(...ys) + pad - y };
  }
  if (it.type === 'text') return { x: it.x, y: it.y, w: Math.max(it.size, it.text.length * it.size * 0.56), h: it.size * 1.25 };
  return { x: it.x, y: it.y, w: it.w, h: it.h };
}

/** Items that can be resized by the handle (and the arrow by its end). */
export const resizable = (it: Item) => it.type === 'rect' || it.type === 'ellipse' || it.type === 'sticky' || it.type === 'image' || it.type === 'arrow';

const MIN = { sticky: 60, rect: 8, ellipse: 8, image: 16 } as const;

/** Resize by dragging the bottom-right handle to `p` (an arrow moves its end); a picture keeps its proportions. */
export function resize(it: Item, p: Pt): Item {
  if (it.type === 'arrow') return { ...it, points: [it.points[0], [round(clampW(p[0])), round(clampW(p[1]))]] };
  if (it.type !== 'rect' && it.type !== 'ellipse' && it.type !== 'sticky' && it.type !== 'image') return it;
  const min = MIN[it.type];
  let w = Math.max(min, p[0] - it.x);
  let h = Math.max(min, p[1] - it.y);
  if (it.type === 'image') h = Math.max(min, (w * it.h) / it.w);
  w = Math.min(8000, w); h = Math.min(8000, h);
  return { ...it, w: round(w), h: round(h) };
}

/** The handle position of a resizable item. */
export function handleAt(it: Item): Pt {
  if (it.type === 'arrow') return it.points[1];
  const b = bbox(it);
  return [b.x + b.w, b.y + b.h];
}

/** A rectangle from a drag in any direction. */
export function boxFrom(a: Pt, b: Pt): Box {
  return { x: round(Math.min(a[0], b[0])), y: round(Math.min(a[1], b[1])), w: round(Math.abs(b[0] - a[0])), h: round(Math.abs(b[1] - a[1])) };
}

export function strokePath(points: Pt[]) {
  if (!points.length) return '';
  const [x0, y0] = points[0];
  if (points.length === 1) return `M${x0} ${y0} l0.01 0`; // a dot
  return `M${x0} ${y0}` + points.slice(1).map(([x, y]) => ` L${x} ${y}`).join('');
}

/** The arrow head: a triangle at the end of the line. */
export function arrowHead([a, b]: Pt[], width: number) {
  const ang = Math.atan2(b[1] - a[1], b[0] - a[0]);
  const len = width * 4 + 12;
  const spread = Math.PI / 7;
  const p1: Pt = [b[0] - len * Math.cos(ang - spread), b[1] - len * Math.sin(ang - spread)];
  const p2: Pt = [b[0] - len * Math.cos(ang + spread), b[1] - len * Math.sin(ang + spread)];
  return `${b[0]},${b[1]} ${p1[0]},${p1[1]} ${p2[0]},${p2[1]}`;
}

/** Zoom to `z` keeping the centre of the view in place. */
export function zoomTo(v: View, z: number): View {
  const cx = v.x + W / v.z / 2;
  const cy = v.y + H / v.z / 2;
  return { x: cx - W / z / 2, y: cy - H / z / 2, z };
}

export const nextZoom = (z: number, dir: 1 | -1) =>
  dir > 0 ? ZOOMS.find((x) => x > z + 1e-6) ?? z : [...ZOOMS].reverse().find((x) => x < z - 1e-6) ?? z;
