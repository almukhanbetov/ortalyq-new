'use client';

import { useMemo, useRef, useState } from 'react';
import { cn } from '@/components/ui';
import type { BoardItem } from '@/types/api';
import { H, INKS, STICKY_FONT, W, arrowHead, bbox, strokePath, withIds, type Item } from './model';

/** One object of the drawing; data-id lets «Выделение» find it under the pointer. */
export function ItemView({ it, imageUrl, hidden }: { it: Item; imageUrl: (imageId: string) => string; hidden?: boolean }) {
  switch (it.type) {
    case 'stroke':
      return (
        <g data-id={it.id} data-item="stroke">
          <path d={strokePath(it.points)} stroke="transparent" strokeWidth={Math.max(16, it.width + 12)} fill="none" strokeLinecap="round" />
          <path d={strokePath(it.points)} stroke={it.color} strokeWidth={it.width} fill="none" strokeLinecap="round" strokeLinejoin="round" />
        </g>
      );
    case 'arrow':
      return (
        <g data-id={it.id} data-item="arrow">
          <line x1={it.points[0][0]} y1={it.points[0][1]} x2={it.points[1][0]} y2={it.points[1][1]} stroke="transparent" strokeWidth={Math.max(16, it.width + 12)} />
          <line x1={it.points[0][0]} y1={it.points[0][1]} x2={it.points[1][0]} y2={it.points[1][1]} stroke={it.color} strokeWidth={it.width} strokeLinecap="round" />
          <polygon points={arrowHead(it.points, it.width)} fill={it.color} />
        </g>
      );
    case 'text':
      return (
        <text data-id={it.id} data-item="text" x={it.x} y={it.y} fill={it.color} fontSize={it.size} dominantBaseline="hanging" fontFamily="inherit"
          style={{ whiteSpace: 'pre', visibility: hidden ? 'hidden' : undefined }}>{it.text}</text>
      );
    case 'sticky':
      return (
        <g data-id={it.id} data-item="sticky">
          <rect x={it.x} y={it.y} width={it.w} height={it.h} rx={10} fill={it.color} className="stroke-line" />
          <foreignObject x={it.x} y={it.y} width={it.w} height={it.h} pointerEvents="none">
            <div style={{ padding: 12, fontSize: STICKY_FONT, lineHeight: 1.3, color: INKS[0].value, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', height: '100%', overflow: 'hidden' }}>{it.text}</div>
          </foreignObject>
        </g>
      );
    case 'rect':
      return <rect data-id={it.id} data-item="rect" x={it.x} y={it.y} width={it.w} height={it.h} rx={4} fill="transparent" stroke={it.color} strokeWidth={it.width} pointerEvents="all" />;
    case 'ellipse':
      return <ellipse data-id={it.id} data-item="ellipse" cx={it.x + it.w / 2} cy={it.y + it.h / 2} rx={it.w / 2} ry={it.h / 2} fill="transparent" stroke={it.color} strokeWidth={it.width} pointerEvents="all" />;
    case 'image':
      return <image data-id={it.id} data-item="image" href={imageUrl(it.image_id)} x={it.x} y={it.y} width={it.w} height={it.h} preserveAspectRatio="none" />;
  }
}


/** Pen drawing on the read-only board (a student with the teacher's permission): one finished stroke at a time. */
export interface ViewerPen {
  color: string;
  width: number;
  onStroke: (points: [number, number][]) => void;
}

/**
 * The board in the lesson for a student: every object; with `pen`, the student draws pen strokes on it (nothing
 * else). The view fits the initial 1600 × 1000 area together with everything drawn outside it. Points are taken
 * through the SVG's own transform, so a stroke starts exactly under the pointer whatever the fit.
 */
export function BoardViewer({ items, imageUrl, label, pen }: { items: BoardItem[]; imageUrl: (imageId: string) => string; label: string; pen?: ViewerPen }) {
  const list = useMemo(() => withIds(items), [items]);
  const ref = useRef<SVGSVGElement>(null);
  const [drawing, setDrawing] = useState<[number, number][] | null>(null);
  const vb = useMemo(() => {
    let x0 = 0, y0 = 0, x1 = W, y1 = H;
    for (const it of list) {
      const b = bbox(it);
      x0 = Math.min(x0, b.x); y0 = Math.min(y0, b.y); x1 = Math.max(x1, b.x + b.w); y1 = Math.max(y1, b.y + b.h);
    }
    const pad = 20;
    return `${x0 - pad} ${y0 - pad} ${x1 - x0 + pad * 2} ${y1 - y0 + pad * 2}`;
  }, [list]);
  const toBoard = (e: { clientX: number; clientY: number }): [number, number] => {
    const svg = ref.current!;
    const pt = svg.createSVGPoint();
    pt.x = e.clientX; pt.y = e.clientY;
    const p = pt.matrixTransform(svg.getScreenCTM()!.inverse());
    return [Math.round(p.x * 10) / 10, Math.round(p.y * 10) / 10];
  };
  const up = () => {
    if (drawing && pen) pen.onStroke(drawing);
    setDrawing(null);
  };
  return (
    <svg ref={ref} viewBox={vb} preserveAspectRatio="xMidYMid meet" role="img" aria-label={label} data-testid="board-viewer"
      className={cn('block aspect-[16/10] w-full select-none rounded-xl border border-line bg-surface shadow-card', pen && 'touch-none cursor-crosshair')}
      onPointerDown={pen ? (e) => { if (e.button !== 0) return; e.currentTarget.setPointerCapture(e.pointerId); setDrawing([toBoard(e)]); } : undefined}
      onPointerMove={pen ? (e) => {
        if (!drawing) return;
        const p = toBoard(e);
        const [lx, ly] = drawing[drawing.length - 1];
        if (Math.hypot(p[0] - lx, p[1] - ly) < 2 || drawing.length >= 4000) return;
        setDrawing([...drawing, p]);
      } : undefined}
      onPointerUp={pen ? up : undefined} onPointerCancel={pen ? up : undefined}>
      {list.map((it) => <ItemView key={it.id} it={it} imageUrl={imageUrl} />)}
      {drawing && pen && <path d={strokePath(drawing)} stroke={pen.color} strokeWidth={pen.width} fill="none" strokeLinecap="round" strokeLinejoin="round" />}
    </svg>
  );
}
