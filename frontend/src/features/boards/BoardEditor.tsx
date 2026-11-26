'use client';

import { useCallback, useEffect, useLayoutEffect, useRef, useState, type MouseEvent as ReactMouseEvent, type PointerEvent as ReactPointerEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle, ArrowLeft, ArrowUpRight, Check, Circle, FileSearch, Hand, ImagePlus, Minus, MousePointer2, Pencil, Plus, Redo2, Save, Square,
  StickyNote, Trash2, Type, Undo2, X,
} from 'lucide-react';
import { Button, EmptyState, ErrorState, Loading, Notice, cn, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { boardsApi } from '@/lib/api/endpoints';
import { ApiError } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { formatDateTime } from '@/lib/format';
import { errorMessage } from '@/lib/errors';
import { boardsUrl } from '@/lib/routes';
import { useMe } from '@/hooks/useMe';
import type { Board } from '@/types/api';
import { ItemView } from './BoardItems';
import { OpenInClassBanner, OpenInClassButton } from './BoardClassControls';
import {
  H, INKS, PEN_WIDTH, STICKY_FONT, STICKY_MAX, TEXT_MAX, TEXT_SIZE, W, bbox, boxFrom, handleAt, newId, nextZoom, resizable, resize, round,
  strokePath, translate, withIds, zoomTo, type Item, type Pt, type View,
} from './model';

type Tool = 'select' | 'hand' | 'pen' | 'text' | 'sticky' | 'rect' | 'ellipse' | 'arrow';

/** «Интерактивные доски» → one board. The server answers 404 to anyone but the owner. */
export function BoardEditor({ id }: { id: string }) {
  const { t } = useI18n();
  const q = useQuery({ queryKey: qk.board(id), queryFn: () => boardsApi.get(id), retry: (n, e) => !(e instanceof ApiError && e.status < 500) && n < 2 });

  if (q.isLoading) return <PageCard maxWidth={1280}><Loading /></PageCard>;
  if (q.error instanceof ApiError && (q.error.status === 404 || q.error.status === 403)) {
    return (
      <PageCard maxWidth={1024}>
        <EmptyState icon={<FileSearch size={26} />} title={t('boards.notFound')} text={t('boards.notFoundHint')}
          action={<Link href={boardsUrl()} className="text-primary-text hover:underline">{t('boards.back')}</Link>} />
      </PageCard>
    );
  }
  if (q.error || !q.data) return <PageCard maxWidth={1024}><ErrorState error={q.error} onRetry={() => q.refetch()} /></PageCard>;
  // Keyed by id: a fresh editor state per board; a refetch after saving does not reset the drawing.
  return <BoardWorkspace key={q.data.id} board={q.data} variant="page" />;
}

/** The document with its history: every committed change pushes the previous items; undo and redo walk the stacks. */
interface Doc { items: Item[]; past: Item[][]; future: Item[][] }
const HISTORY = 200;

/** What the pointer is doing right now (not part of the history until it ends). */
type Gesture =
  | { kind: 'pen'; points: Pt[] }
  | { kind: 'pan'; sx: number; sy: number; view: View }
  | { kind: 'move'; id: string; from: Pt; orig: Item; moved: boolean }
  | { kind: 'resize'; id: string; orig: Item }
  | { kind: 'shape'; type: 'rect' | 'ellipse' | 'arrow'; from: Pt };

/** Text being typed: a new text at (x, y), or an existing text / sticky being edited. */
type Typing = { kind: 'new-text'; x: number; y: number; value: string } | { kind: 'edit'; id: string; value: string };

/**
 * The editor of one board. `page`: the board page in «Материалы» (with «Открыть в классе»). `class`: embedded in
 * the lesson for the teacher of the class. While the board is open in a class every change is saved automatically
 * (a moment after the teacher stops), so the students see it without reloading.
 */
export function BoardWorkspace({ board, variant, onCloseInClass, closing }: {
  board: Board; variant: 'page' | 'class'; onCloseInClass?: () => void; closing?: boolean;
}) {
  const { t, locale } = useI18n();
  const me = useMe().data;
  const toast = useToast();
  const qc = useQueryClient();
  const [initial] = useState(() => withIds(board.content.items));
  const [doc, setDoc] = useState<Doc>({ items: initial, past: [], future: [] });
  const [saved, setSaved] = useState<Item[]>(initial); // what the server has
  const [version, setVersion] = useState(board.version);
  const [updatedAt, setUpdatedAt] = useState(board.updated_at);
  const [tool, setTool] = useState<Tool>('pen');
  const [ink, setInk] = useState<(typeof INKS)[number]>(INKS[0]);
  const [view, setView] = useState<View>({ x: 0, y: 0, z: 1 });
  const [selected, setSelected] = useState<string | null>(null);
  const [gesture, setGesture] = useState<Gesture | null>(null);
  const [preview, setPreview] = useState<Item | null>(null); // the moved/resized/new item while the pointer is down
  const [typing, setTypingState] = useState<Typing | null>(null);
  // Mirrored in a ref: a click on the canvas and the input's blur may both finish the same typing; only one commits.
  const typingRef = useRef<Typing | null>(null);
  const setTyping = (v: Typing | null) => { typingRef.current = v; setTypingState(v); };
  const [uploading, setUploading] = useState(false);
  const svgRef = useRef<SVGSVGElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const [cw, setCw] = useState(1); // the canvas width in screen pixels (for the HTML overlays)

  const items = doc.items;
  const dirty = items !== saved;
  // For merging strokes of others (a student drawing in the lesson): every object id this editor has ever had.
  // A remote object with an unseen id is new and joins the drawing; one the teacher deleted stays deleted.
  const seen = useRef(new Set(initial.map((x) => x.id)));
  const docRef = useRef(doc);
  docRef.current = doc;
  const savedRef = useRef(saved);
  savedRef.current = saved;
  useEffect(() => { for (const x of items) seen.current.add(x.id); }, [items]);
  const sel = items.find((x) => x.id === selected) ?? null;

  useLayoutEffect(() => {
    const el = svgRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setCw(el.getBoundingClientRect().width || 1));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // ---- history ----
  const commit = useCallback((next: (xs: Item[]) => Item[]) => {
    setDoc((d) => {
      const items2 = next(d.items);
      if (items2 === d.items) return d;
      return { items: items2, past: [...d.past.slice(-HISTORY + 1), d.items], future: [] };
    });
  }, []);
  const undo = useCallback(() => setDoc((d) => (d.past.length
    ? { items: d.past[d.past.length - 1], past: d.past.slice(0, -1), future: [d.items, ...d.future] } : d)), []);
  const redo = useCallback(() => setDoc((d) => (d.future.length
    ? { items: d.future[0], past: [...d.past, d.items], future: d.future.slice(1) } : d)), []);
  const remove = useCallback((id: string) => { commit((xs) => xs.filter((x) => x.id !== id)); setSelected(null); }, [commit]);

  // ---- saving (errors keep the drawing; a conflict offers «Сохранить мою версию») ----
  /**
   * Adds objects of the server's board that this editor has never seen (a student's strokes) to the drawing and to
   * every undo/redo step, and takes the server's version, so the next save keeps both sides' work.
   */
  const merge = useCallback((remote: Board) => {
    const fresh = remote.content.items.filter((x): x is Item => !!x.id && !seen.current.has(x.id));
    for (const x of fresh) seen.current.add(x.id);
    setVersion((v) => Math.max(v, remote.version));
    if (!fresh.length) return;
    const d = docRef.current;
    const next = [...d.items, ...fresh];
    setDoc({ items: next, past: d.past.map((xs) => [...xs, ...fresh]), future: d.future.map((xs) => [...xs, ...fresh]) });
    if (d.items === savedRef.current) setSaved(next); // nothing of the teacher's own was waiting: still saved
  }, []);

  const save = useMutation({
    mutationFn: ({ snapshot, force }: { snapshot: Item[]; force?: boolean }) =>
      boardsApi.save(board.id, { content: { items: snapshot }, version, force }),
    onSuccess: (b, { snapshot }) => {
      setSaved(snapshot); // drawing made while saving stays unsaved (items !== snapshot)
      setVersion(b.version);
      setUpdatedAt(b.updated_at);
      qc.setQueryData(qk.board(board.id), b);
      qc.invalidateQueries({ queryKey: ['boards'] });
    },
  });
  const conflict = save.error instanceof ApiError && save.error.status === 409;
  // With autosave a conflict is merged by itself (see below), so it is not shown as an error.
  const showError = save.isError && !(conflict && (variant === 'class' || (board.open_in ?? []).length > 0));
  const doSave = useCallback((force?: boolean) => {
    if (save.isPending) return;
    save.mutate({ snapshot: items, force });
  }, [items, save]);
  const imageUrl = useCallback((imageId: string) => boardsApi.imageUrl(board.id, imageId), [board.id]);

  // Open in a class: save by itself a moment after a change. A failed save stops it until «Повторить сохранение».
  const openIn = board.open_in ?? [];
  const autosave = variant === 'class' || openIn.length > 0;

  // In the lesson the board is re-read after every change (a student's stroke): take in what is new.
  useEffect(() => {
    if (variant === 'class' && board.version > version) merge(board);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [board, variant, merge]);
  // A save refused because a student drew meanwhile (409): merge the server's board and save again by itself.
  const conflictRebase = useRef(false);
  useEffect(() => {
    if (!autosave || !(save.error instanceof ApiError) || save.error.status !== 409 || conflictRebase.current) return;
    conflictRebase.current = true;
    boardsApi.get(board.id).then((b) => { merge(b); save.reset(); }).catch(() => {}).finally(() => { conflictRebase.current = false; });
  }, [autosave, save.error, save, board.id, merge]);
  useEffect(() => {
    if (!autosave || !dirty || save.isPending || save.isError || gesture || typing) return;
    const id = setTimeout(() => doSave(), 700);
    return () => clearTimeout(id);
  }, [autosave, dirty, items, save.isPending, save.isError, gesture, typing, doSave]);

  // ---- keyboard: Ctrl+Z / Ctrl+Shift+Z (Ctrl+Y), Ctrl+S, Delete, Escape (not while typing) ----
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      if (typing || (el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA'))) return;
      const k = e.key.toLowerCase();
      if (e.ctrlKey || e.metaKey) {
        if (k === 'z' && !e.shiftKey) { e.preventDefault(); undo(); }
        else if ((k === 'z' && e.shiftKey) || k === 'y') { e.preventDefault(); redo(); }
        else if (k === 's') { e.preventDefault(); doSave(); }
        return;
      }
      if ((e.key === 'Delete' || e.key === 'Backspace') && selected) { e.preventDefault(); remove(selected); }
      if (e.key === 'Escape') setSelected(null);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [undo, redo, doSave, remove, selected, typing]);

  // Unsaved changes are not lost by an accidental close or reload.
  useEffect(() => {
    if (!dirty) return;
    const onLeave = (e: BeforeUnloadEvent) => { e.preventDefault(); e.returnValue = ''; };
    window.addEventListener('beforeunload', onLeave);
    return () => window.removeEventListener('beforeunload', onLeave);
  }, [dirty]);

  // ---- coordinates: the visible area is (view.x, view.y) .. + (W, H) / zoom ----
  const vw = W / view.z;
  const vh = H / view.z;
  const toBoard = (e: { clientX: number; clientY: number }): Pt => {
    const r = svgRef.current!.getBoundingClientRect();
    return [round(view.x + ((e.clientX - r.left) / r.width) * vw), round(view.y + ((e.clientY - r.top) / r.height) * vh)];
  };
  const px = cw / vw; // screen pixels per board unit
  const overlay = (x: number, y: number) => ({ left: `${((x - view.x) / vw) * 100}%`, top: `${((y - view.y) / vh) * 100}%` });

  // ---- typing ----
  const commitTyping = () => {
    const typing = typingRef.current;
    if (!typing) return;
    setTyping(null);
    const value = typing.value;
    if (typing.kind === 'new-text') {
      const text = value.trim();
      if (text) commit((xs) => [...xs, { id: newId(), type: 'text', color: ink.value, x: typing.x, y: typing.y, size: TEXT_SIZE, text }]);
    } else {
      commit((xs) => {
        const it = xs.find((x) => x.id === typing.id);
        if (!it || (it.type !== 'text' && it.type !== 'sticky')) return xs;
        if (it.type === 'text' && !value.trim()) return xs.filter((x) => x.id !== it.id); // an emptied text is removed
        const next = it.type === 'text' ? value.trim() : value;
        return next === it.text ? xs : xs.map((x) => (x.id === it.id ? { ...it, text: next } : x));
      });
    }
  };

  const pick = (e: ReactPointerEvent): { id: string | null; handle: boolean } => {
    const el = (e.target as Element).closest('[data-id],[data-handle]');
    if (!el) return { id: null, handle: false };
    if (el.hasAttribute('data-handle')) return { id: el.getAttribute('data-handle'), handle: true };
    return { id: el.getAttribute('data-id'), handle: false };
  };

  const onPointerDown = (e: ReactPointerEvent<SVGSVGElement>) => {
    if (e.button !== 0 && e.button !== 1) return;
    if (typing) { commitTyping(); if (tool !== 'text') return; }
    const p = toBoard(e);
    const capture = () => e.currentTarget.setPointerCapture(e.pointerId);
    if (tool === 'hand' || e.button === 1) {
      capture();
      setGesture({ kind: 'pan', sx: e.clientX, sy: e.clientY, view });
      return;
    }
    if (tool === 'select') {
      const hit = pick(e);
      const it = hit.id ? items.find((x) => x.id === hit.id) : undefined;
      if (!it) { setSelected(null); return; }
      setSelected(it.id);
      capture();
      setGesture(hit.handle ? { kind: 'resize', id: it.id, orig: it } : { kind: 'move', id: it.id, from: p, orig: it, moved: false });
      return;
    }
    setSelected(null);
    if (tool === 'text') {
      e.preventDefault();
      setTyping({ kind: 'new-text', x: p[0], y: p[1], value: '' });
      return;
    }
    if (tool === 'sticky') {
      e.preventDefault();
      const id = newId();
      commit((xs) => [...xs, { id, type: 'sticky', color: ink.tint, x: p[0], y: p[1], w: 240, h: 180, text: '' }]);
      setSelected(id);
      setTyping({ kind: 'edit', id, value: '' });
      return;
    }
    capture();
    if (tool === 'pen') setGesture({ kind: 'pen', points: [p] });
    else setGesture({ kind: 'shape', type: tool, from: p });
  };

  const onPointerMove = (e: ReactPointerEvent<SVGSVGElement>) => {
    if (!gesture) return;
    if (gesture.kind === 'pan') {
      const r = svgRef.current!.getBoundingClientRect();
      const g = gesture.view;
      setView({ ...g, x: g.x - ((e.clientX - gesture.sx) / r.width) * (W / g.z), y: g.y - ((e.clientY - gesture.sy) / r.height) * (H / g.z) });
      return;
    }
    const p = toBoard(e);
    if (gesture.kind === 'pen') {
      const pts = gesture.points;
      const [lx, ly] = pts[pts.length - 1];
      if (Math.hypot(p[0] - lx, p[1] - ly) < 2 / view.z || pts.length >= 4000) return;
      setGesture({ kind: 'pen', points: [...pts, p] });
    } else if (gesture.kind === 'move') {
      const dx = p[0] - gesture.from[0];
      const dy = p[1] - gesture.from[1];
      if (!gesture.moved && Math.hypot(dx, dy) * view.z < 3) return;
      if (!gesture.moved) setGesture({ ...gesture, moved: true });
      setPreview(translate(gesture.orig, dx, dy));
    } else if (gesture.kind === 'resize') {
      setPreview(resize(gesture.orig, p));
    } else if (gesture.kind === 'shape') {
      setPreview(shapeItem(gesture.type, gesture.from, p, ink.value, 'preview'));
    }
  };

  const onPointerUp = (e: ReactPointerEvent<SVGSVGElement>) => {
    const g = gesture;
    setGesture(null);
    const pv = preview;
    setPreview(null);
    if (!g) return;
    if (g.kind === 'pen') {
      commit((xs) => [...xs, { id: newId(), type: 'stroke', color: ink.value, width: PEN_WIDTH, points: g.points }]);
    } else if ((g.kind === 'move' && g.moved) || g.kind === 'resize') {
      if (pv) commit((xs) => xs.map((x) => (x.id === g.id ? pv : x)));
    } else if (g.kind === 'shape') {
      const p = toBoard(e);
      const it = shapeItem(g.type, g.from, p, ink.value, newId());
      commit((xs) => [...xs, it]);
    }
  };

  const onDoubleClick = (e: ReactMouseEvent<SVGSVGElement>) => {
    if (tool !== 'select') return;
    // The pointer was captured by the canvas while the object was pressed, so the event's target is the canvas:
    // find the object under the point instead.
    const el = document.elementsFromPoint(e.clientX, e.clientY).map((x) => x.closest('[data-id]')).find(Boolean) ?? null;
    const it = el ? items.find((x) => x.id === el.getAttribute('data-id')) : undefined;
    if (it && (it.type === 'text' || it.type === 'sticky')) { setSelected(it.id); setTyping({ kind: 'edit', id: it.id, value: it.text }); }
  };

  // A colour applies to new objects; with «Выделение» it also recolours the selected one (a sticky gets the matching
  // paper). With a drawing tool it never touches the object made just before.
  const chooseInk = (c: (typeof INKS)[number]) => {
    setInk(c);
    if (tool === 'select' && sel && sel.type !== 'image') {
      const color = sel.type === 'sticky' ? c.tint : c.value;
      if (sel.color !== color) commit((xs) => xs.map((x) => (x.id === sel.id ? ({ ...x, color } as Item) : x)));
    }
  };

  const onImage = async (file: File | undefined) => {
    if (!file) return;
    setUploading(true);
    try {
      const img = await boardsApi.uploadImage(board.id, file);
      const scale = Math.min(1, (vw * 0.4) / img.width, (vh * 0.5) / img.height);
      const w = round(Math.max(16, img.width * scale));
      const h = round(Math.max(16, img.height * scale));
      const id = newId();
      commit((xs) => [...xs, { id, type: 'image', image_id: img.id, x: round(view.x + (vw - w) / 2), y: round(view.y + (vh - h) / 2), w, h }]);
      setTool('select');
      setSelected(id);
    } catch (err) {
      toast(`${t('boards.imageFailed')} ${errorMessage(err, t)}`, 'error');
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  const shown = preview && items.some((x) => x.id === preview.id) ? items.map((x) => (x.id === preview.id ? preview : x)) : items;
  const selShown = sel ? shown.find((x) => x.id === sel.id) ?? null : null;
  const status = save.isPending ? t('boards.saving') : dirty ? t('boards.unsaved') : t('boards.saved');
  const editing = typing?.kind === 'edit' ? items.find((x) => x.id === typing.id) : undefined;
  const cursor = { select: 'cursor-default', hand: gesture?.kind === 'pan' ? 'cursor-grabbing' : 'cursor-grab', pen: 'cursor-crosshair', text: 'cursor-text',
    sticky: 'cursor-copy', rect: 'cursor-crosshair', ellipse: 'cursor-crosshair', arrow: 'cursor-crosshair' }[tool];

  const page = variant === 'page';
  const body = (
    <>
      {page && <Link href={boardsUrl()} className="inline-flex items-center gap-1 text-sm text-ink-muted hover:text-ink-body"><ArrowLeft size={16} />{t('boards.back')}</Link>}
      <div className={cn('flex flex-wrap items-end justify-between gap-3', page && 'mt-2')}>
        <div className="min-w-0">
          {page ? <h1 className="truncate text-[26px] font-bold leading-8 text-ink-body">{board.title}</h1>
            : <h2 className="truncate text-xl font-bold text-ink-body">{t('boards.inClassTitle', { title: board.title })}</h2>}
          <p className="text-sm text-ink-faint">{t('boards.updated', { date: formatDateTime(updatedAt, locale, me?.timezone) })}</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <p className={cn('inline-flex items-center gap-1.5 text-sm', dirty ? 'text-warning-text' : 'text-ink-muted')} aria-live="polite" data-testid="board-status">
            {!dirty && !save.isPending && <Check size={16} />}{status}
          </p>
          {page && <OpenInClassButton boardId={board.id} />}
          {!page && onCloseInClass && (
            <Button size="sm" variant="secondary" icon={<X size={16} />} loading={closing} onClick={onCloseInClass}>{t('boards.closeInClass')}</Button>
          )}
        </div>
      </div>
      {page && openIn.length > 0 && <OpenInClassBanner boardId={board.id} classes={openIn} />}
      {!page && <p className="mt-1 text-sm text-ink-muted">{t('boards.autosaveHint')}</p>}

      <div role="toolbar" aria-label={t('boards.tools')} data-testid="board-toolbar" className="mt-4 flex flex-wrap items-center gap-1.5 rounded-xl border border-line bg-surface-alt p-2">
        <ToolButton tool="select" current={tool} set={setTool} icon={<MousePointer2 size={18} />} label={t('boards.select')} />
        <ToolButton tool="hand" current={tool} set={setTool} icon={<Hand size={18} />} label={t('boards.hand')} />
        <Sep />
        <ToolButton tool="pen" current={tool} set={setTool} icon={<Pencil size={18} />} label={t('boards.pen')} />
        <ToolButton tool="text" current={tool} set={setTool} icon={<Type size={18} />} label={t('boards.text')} />
        <ToolButton tool="sticky" current={tool} set={setTool} icon={<StickyNote size={18} />} label={t('boards.sticky')} />
        <ToolButton tool="rect" current={tool} set={setTool} icon={<Square size={18} />} label={t('boards.rect')} />
        <ToolButton tool="ellipse" current={tool} set={setTool} icon={<Circle size={18} />} label={t('boards.ellipse')} />
        <ToolButton tool="arrow" current={tool} set={setTool} icon={<ArrowUpRight size={18} />} label={t('boards.arrow')} />
        <IconAction label={uploading ? t('boards.imageUploading') : t('boards.image')} disabled={uploading} onClick={() => fileRef.current?.click()}><ImagePlus size={18} /></IconAction>
        <input ref={fileRef} type="file" accept="image/png,image/jpeg,image/webp" className="sr-only" tabIndex={-1} aria-hidden data-testid="board-image-input"
          onChange={(e) => onImage(e.target.files?.[0])} />
        <Sep />
        <div role="radiogroup" aria-label={t('boards.color')} className="flex items-center gap-1">
          {INKS.map((c) => (
            <button key={c.value} type="button" role="radio" aria-checked={ink.value === c.value} aria-label={t(`boards.colors.${c.name}`)} title={t(`boards.colors.${c.name}`)}
              onClick={() => chooseInk(c)}
              className={cn('h-7 w-7 rounded-full border-2 transition-transform focus-visible:outline-none focus-visible:shadow-focus',
                ink.value === c.value ? 'scale-110 border-ink' : 'border-surface hover:scale-105')}
              style={{ backgroundColor: c.value }} />
          ))}
        </div>
        <Sep />
        <IconAction label={t('boards.undo')} disabled={!doc.past.length} onClick={undo}><Undo2 size={18} /></IconAction>
        <IconAction label={t('boards.redo')} disabled={!doc.future.length} onClick={redo}><Redo2 size={18} /></IconAction>
        <IconAction label={t('boards.delete')} disabled={!sel} onClick={() => sel && remove(sel.id)}><Trash2 size={18} /></IconAction>
        <Sep />
        <div role="group" aria-label={t('boards.zoom')} className="flex items-center gap-1">
          <IconAction label={t('boards.zoomOut')} disabled={view.z <= 0.25} onClick={() => setView((v) => zoomTo(v, nextZoom(v.z, -1)))}><Minus size={18} /></IconAction>
          <button type="button" onClick={() => setView((v) => zoomTo(v, 1))} title={t('boards.zoomReset')} aria-label={t('boards.zoomResetLabel', { n: Math.round(view.z * 100) })}
            data-testid="board-zoom"
            className="h-9 min-w-[64px] rounded-lg bg-surface px-2 text-sm font-medium tabular-nums text-ink-body hover:bg-surface-hover focus-visible:outline-none focus-visible:shadow-focus">
            {Math.round(view.z * 100)}%
          </button>
          <IconAction label={t('boards.zoomIn')} disabled={view.z >= 4} onClick={() => setView((v) => zoomTo(v, nextZoom(v.z, 1)))}><Plus size={18} /></IconAction>
        </div>
        <div className="ml-auto">
          <Button size="sm" icon={<Save size={16} />} loading={save.isPending} disabled={!dirty && !save.isError} onClick={() => doSave()}>{t('common.save')}</Button>
        </div>
      </div>

      {showError && (
        <Notice tone={conflict ? 'warning' : 'danger'} icon={<AlertTriangle size={18} />} className="mt-3"
          action={conflict
            ? <Button size="sm" variant="secondary" loading={save.isPending} onClick={() => doSave(true)}>{t('boards.overwrite')}</Button>
            : <Button size="sm" variant="secondary" loading={save.isPending} onClick={() => doSave()}>{t('boards.retry')}</Button>}>
          <span className="flex-1 text-sm">{conflict ? t('boards.conflict') : `${t('boards.saveFailed')} ${errorMessage(save.error, t)}`}</span>
        </Notice>
      )}

      <div className="relative mt-3 overflow-hidden rounded-xl border border-line bg-surface shadow-card">
        <svg ref={svgRef} viewBox={`${view.x} ${view.y} ${vw} ${vh}`} role="img" aria-label={t('boards.canvas')} data-testid="board-canvas"
          className={cn('block aspect-[16/10] w-full touch-none select-none', cursor)}
          onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} onPointerCancel={onPointerUp} onDoubleClick={onDoubleClick}>
          {shown.map((it) => <ItemView key={it.id} it={it} imageUrl={imageUrl} hidden={editing?.id === it.id && it.type === 'text'} />)}
          {gesture?.kind === 'pen' && <path d={strokePath(gesture.points)} stroke={ink.value} strokeWidth={PEN_WIDTH} fill="none" strokeLinecap="round" strokeLinejoin="round" />}
          {preview && !items.some((x) => x.id === preview.id) && <ItemView it={preview} imageUrl={imageUrl} />}
          {selShown && tool === 'select' && <Selection it={selShown} z={view.z} />}
        </svg>
        {typing?.kind === 'new-text' && (
          <input autoFocus aria-label={t('boards.textInput')} maxLength={TEXT_MAX} value={typing.value}
            onChange={(e) => setTyping({ ...typing, value: e.target.value })}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commitTyping(); } if (e.key === 'Escape') setTyping(null); }}
            onBlur={commitTyping}
            className="absolute min-w-[160px] rounded-md border-2 border-primary bg-surface/90 px-1 focus:outline-none"
            style={{ ...overlay(typing.x, typing.y), color: ink.value, fontSize: Math.max(12, TEXT_SIZE * px) }} />
        )}
        {typing?.kind === 'edit' && editing && editing.type === 'text' && (
          <input autoFocus aria-label={t('boards.textInput')} maxLength={TEXT_MAX} value={typing.value}
            onChange={(e) => setTyping({ ...typing, value: e.target.value })}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commitTyping(); } if (e.key === 'Escape') setTyping(null); }}
            onBlur={commitTyping}
            className="absolute min-w-[160px] rounded-md border-2 border-primary bg-surface/90 px-1 focus:outline-none"
            style={{ ...overlay(editing.x, editing.y), color: editing.color, fontSize: Math.max(12, editing.size * px) }} />
        )}
        {typing?.kind === 'edit' && editing && editing.type === 'sticky' && (
          <textarea autoFocus aria-label={t('boards.stickyInput')} maxLength={STICKY_MAX} value={typing.value}
            onChange={(e) => setTyping({ ...typing, value: e.target.value })}
            onKeyDown={(e) => { if (e.key === 'Escape') setTyping(null); if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); commitTyping(); } }}
            onBlur={commitTyping}
            className="absolute resize-none rounded-md border-2 border-primary p-2 focus:outline-none"
            style={{ ...overlay(editing.x, editing.y), color: INKS[0].value, width: editing.w * px, height: editing.h * px, backgroundColor: editing.color, fontSize: Math.max(10, STICKY_FONT * px) }} />
        )}
      </div>
      <p className="mt-2 text-xs text-ink-muted">{t(`boards.hints.${tool}`)}</p>
    </>
  );
  // A plain choice of container (not a component made during render, which would remount the canvas every time).
  return page
    ? <PageCard maxWidth={1280} className="pt-[18px] sm:pt-[18px]">{body}</PageCard>
    : <section aria-label={board.title}>{body}</section>;
}

function shapeItem(type: 'rect' | 'ellipse' | 'arrow', a: Pt, b: Pt, color: string, id: string): Item {
  const small = Math.hypot(b[0] - a[0], b[1] - a[1]) < 6;
  if (type === 'arrow') return { id, type, color, width: PEN_WIDTH, points: [a, small ? [round(a[0] + 200), a[1]] : b] };
  const box = small ? { x: a[0], y: a[1], w: 200, h: 120 } : boxFrom(a, b);
  return { id, type, color, width: PEN_WIDTH, x: box.x, y: box.y, w: Math.max(8, box.w), h: Math.max(8, box.h) };
}

/** The dashed frame of the selected object and its resize handle (the arrow's end for an arrow). */
function Selection({ it, z }: { it: Item; z: number }) {
  const b = bbox(it);
  const pad = 6 / z;
  const hs = 14 / z;
  const [hx, hy] = handleAt(it);
  return (
    <g pointerEvents="none" data-testid="board-selection">
      <rect x={b.x - pad} y={b.y - pad} width={b.w + pad * 2} height={b.h + pad * 2} fill="none" className="stroke-primary" strokeWidth={2 / z} strokeDasharray={`${6 / z} ${4 / z}`} />
      {resizable(it) && (
        <rect data-handle={it.id} data-testid="board-resize-handle" x={hx - hs / 2} y={hy - hs / 2} width={hs} height={hs} rx={3 / z}
          className="fill-surface stroke-primary" strokeWidth={2 / z} pointerEvents="all" style={{ cursor: 'nwse-resize' }} />
      )}
    </g>
  );
}

function ToolButton({ tool, current, set, icon, label }: { tool: Tool; current: Tool; set: (t: Tool) => void; icon: ReactNode; label: string }) {
  const active = tool === current;
  return (
    <button type="button" aria-pressed={active} aria-label={label} title={label} onClick={() => set(tool)}
      className={cn('inline-flex h-9 w-9 items-center justify-center rounded-lg transition-colors focus-visible:outline-none focus-visible:shadow-focus',
        active ? 'bg-primary text-white' : 'bg-surface text-ink-body hover:bg-surface-hover')}>
      {icon}
    </button>
  );
}

function IconAction({ label, disabled, onClick, children }: { label: string; disabled?: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button type="button" aria-label={label} title={label} disabled={disabled} onClick={onClick}
      className="inline-flex h-9 w-9 items-center justify-center rounded-lg bg-surface text-ink-body transition-colors hover:bg-surface-hover focus-visible:outline-none focus-visible:shadow-focus disabled:cursor-not-allowed disabled:opacity-40">
      {children}
    </button>
  );
}

function Sep() {
  return <span className="mx-0.5 h-6 w-px bg-line" aria-hidden />;
}
