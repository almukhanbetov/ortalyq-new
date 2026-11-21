'use client';

import { useEffect, useState, useSyncExternalStore } from 'react';
import { useHoverExpand } from './useHoverExpand';

const KEY = 'ortalyq_sidebar_pinned';

// ---- Pinned mode: one shared store for the whole cabinet (survives page changes, syncs across tabs).
const listeners = new Set<() => void>();
function readPinned() {
  try {
    return localStorage.getItem(KEY) === '1';
  } catch {
    return false;
  }
}
function subscribe(fn: () => void) {
  listeners.add(fn);
  const onStorage = (e: StorageEvent) => e.key === KEY && fn();
  window.addEventListener('storage', onStorage);
  return () => {
    listeners.delete(fn);
    window.removeEventListener('storage', onStorage);
  };
}
function setPinnedStored(v: boolean) {
  try {
    localStorage.setItem(KEY, v ? '1' : '0');
  } catch {
    /* storage unavailable: state still applies for this page */
  }
  listeners.forEach((l) => l());
}

/**
 * Cabinet sidebar state.
 * - narrow by default; expands while hovered by a mouse/pen or while keyboard focus is inside;
 * - collapses HOVER_CLOSE_DELAY_MS after the pointer leaves (see useHoverExpand);
 * - pinned = always expanded, content shifts; remembered per browser.
 */
export function useSidebar(panel: React.RefObject<HTMLElement | null>) {
  const pinned = useSyncExternalStore(subscribe, readPinned, () => false);
  const hover = useHoverExpand(panel);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    // Enable width animation only after the stored mode is applied (no slide on load).
    const id = requestAnimationFrame(() => setReady(true));
    return () => cancelAnimationFrame(id);
  }, []);

  return {
    pinned,
    expanded: pinned || hover.expanded,
    peeking: !pinned && hover.expanded,
    ready,
    togglePin: () => setPinnedStored(!readPinned()),
    panelProps: hover.panelProps,
  };
}
