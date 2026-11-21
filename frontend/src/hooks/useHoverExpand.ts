'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

export const HOVER_CLOSE_DELAY_MS = 180;

// Input modality: keyboard focus may expand a rail, focus caused by mouse clicks may not.
let keyboardModality = false;
if (typeof window !== 'undefined') {
  window.addEventListener('keydown', (e) => {
    if (e.key === 'Tab' || e.key.startsWith('Arrow')) keyboardModality = true;
  }, true);
  window.addEventListener('pointerdown', () => { keyboardModality = false; }, true);
}

/**
 * Hover/keyboard-focus expansion for a narrow rail (cabinet menu, classroom toolbar).
 * Opens on mouse/pen hover or keyboard focus inside; closes HOVER_CLOSE_DELAY_MS after the
 * pointer leaves, immediately when focus leaves. Touch input is ignored (tap-only on phones).
 */
export function useHoverExpand(panel: React.RefObject<HTMLElement | null>) {
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => () => clearTimeout(closeTimer.current), []);

  const open = useCallback(() => {
    clearTimeout(closeTimer.current);
    setHovered(true);
  }, []);
  const scheduleClose = useCallback(() => {
    clearTimeout(closeTimer.current);
    closeTimer.current = setTimeout(() => setHovered(false), HOVER_CLOSE_DELAY_MS);
  }, []);

  // Safety net for lost boundary events while open.
  useEffect(() => {
    if (!hovered) return;
    const onMove = (e: PointerEvent) => {
      if (e.pointerType !== 'touch' && !panel.current?.contains(e.target as Node)) scheduleClose();
    };
    const onLeaveWindow = () => scheduleClose();
    document.addEventListener('pointermove', onMove, { passive: true });
    document.documentElement.addEventListener('pointerleave', onLeaveWindow);
    window.addEventListener('blur', onLeaveWindow);
    return () => {
      document.removeEventListener('pointermove', onMove);
      document.documentElement.removeEventListener('pointerleave', onLeaveWindow);
      window.removeEventListener('blur', onLeaveWindow);
    };
  }, [hovered, panel, scheduleClose]);

  const panelProps = {
    onPointerEnter: (e: React.PointerEvent) => { if (e.pointerType !== 'touch') open(); },
    // The cursor may already rest over the rail after load/navigation (no enter event).
    onPointerMove: (e: React.PointerEvent) => { if (e.pointerType !== 'touch' && !hovered) open(); },
    onPointerLeave: (e: React.PointerEvent) => { if (e.pointerType !== 'touch') scheduleClose(); },
    onFocus: () => { if (keyboardModality) setFocused(true); },
    onBlur: (e: React.FocusEvent<HTMLElement>) => {
      if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFocused(false);
    },
    onPointerDown: () => setFocused(false),
  };

  return { expanded: hovered || focused, panelProps };
}
