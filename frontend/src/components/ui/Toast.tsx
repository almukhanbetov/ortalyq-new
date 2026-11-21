'use client';

import { createContext, useCallback, useContext, useState, type ReactNode } from 'react';
import { CheckCircle2, AlertCircle, X } from 'lucide-react';
import { cn } from './cn';

type ToastTone = 'success' | 'error' | 'info';
interface Toast { id: number; tone: ToastTone; text: string }

const Ctx = createContext<(text: string, tone?: ToastTone) => void>(() => {});

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([]);
  const push = useCallback((text: string, tone: ToastTone = 'info') => {
    const id = Date.now() + Math.random();
    setItems((x) => [...x, { id, tone, text }]);
    setTimeout(() => setItems((x) => x.filter((t) => t.id !== id)), 4500);
  }, []);
  return (
    <Ctx.Provider value={push}>
      {children}
      <div aria-live="polite" className="pointer-events-none fixed bottom-4 left-1/2 z-[60] flex w-[min(92vw,420px)] -translate-x-1/2 flex-col gap-2">
        {items.map((t) => (
          <div key={t.id} role={t.tone === 'error' ? 'alert' : 'status'}
            className={cn('pointer-events-auto flex items-start gap-3 rounded-xl px-4 py-3 text-base shadow-pop animate-pop-in',
              t.tone === 'error' ? 'bg-ink text-white' : 'bg-ink text-white')}>
            {t.tone === 'success' ? <CheckCircle2 size={20} className="shrink-0 text-success" /> : t.tone === 'error' ? <AlertCircle size={20} className="shrink-0 text-danger" /> : null}
            <span className="flex-1">{t.text}</span>
            <button type="button" aria-label="×" onClick={() => setItems((x) => x.filter((i) => i.id !== t.id))} className="text-white/60 hover:text-white"><X size={16} /></button>
          </div>
        ))}
      </div>
    </Ctx.Provider>
  );
}

export const useToast = () => useContext(Ctx);
