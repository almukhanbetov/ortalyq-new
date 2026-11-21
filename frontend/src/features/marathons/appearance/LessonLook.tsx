'use client';

import { useCallback, useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Volume2, VolumeX } from 'lucide-react';
import { cn } from '@/components/ui';
import { marathonsApi } from '@/lib/api/endpoints';
import { attachmentUrl } from '@/lib/api/client';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import type { EffectiveAppearance } from '@/types/api';
import { BACKGROUND_CLASS, iconOf } from './reactions';
import { playSound, unlockAudio } from './sounds';

/** The appearance applied on a marathon lesson page (student or owner preview); re-read on marathon.changed. */
export function useLessonLook(marathonId: string, lessonId: string, enabled: boolean) {
  return useQuery({
    queryKey: qk.marathonEffectiveAppearance(marathonId, lessonId),
    queryFn: () => marathonsApi.effectiveAppearance(marathonId, lessonId),
    enabled,
    retry: false,
  }).data;
}

/**
 * Background layer of the lesson card. The card must be `relative isolate` (and clip its corners with overflow-clip,
 * which keeps sticky banners working). Ready variants are CSS classes built from tokens; a custom picture is covered
 * with a light veil so text stays readable.
 */
export function LookBackground({ look }: { look: EffectiveAppearance | undefined }) {
  const bg = look?.background;
  if (!bg) return null;
  if (bg.preset === 'custom' && bg.image_id) {
    return (
      <div aria-hidden className="pointer-events-none absolute inset-0 -z-10" data-testid="lesson-background" data-preset="custom">
        <div className="absolute inset-0 bg-cover bg-center" style={{ backgroundImage: `url("${attachmentUrl(bg.image_id, true)}")` }} />
        <div className="absolute inset-0 bg-surface/60" />
      </div>
    );
  }
  return <div aria-hidden className={cn('pointer-events-none absolute inset-0 -z-10', BACKGROUND_CLASS[bg.preset as keyof typeof BACKGROUND_CLASS])} data-testid="lesson-background" data-preset={bg.preset} />;
}

const SOUND_KEY = 'ortalyq.marathon.reactionSound';

/** «Звук реакций» of this browser (a per-viewer convenience; storage may be unavailable). */
function useSoundPreference(): [boolean, (v: boolean) => void] {
  const [on, setOn] = useState(true);
  useEffect(() => {
    try {
      if (window.localStorage.getItem(SOUND_KEY) === 'off') setOn(false);
    } catch { /* storage blocked: keep the default */ }
  }, []);
  const set = useCallback((v: boolean) => {
    setOn(v);
    try {
      window.localStorage.setItem(SOUND_KEY, v ? 'on' : 'off');
    } catch { /* storage blocked */ }
  }, []);
  return [on, set];
}

type Burst = { key: number; icon: string; correct: boolean };

/**
 * Reactions to answers. `react` is called by the test player only for an answer the server has just checked
 * (instant check), with the server's verdict — never on the client's own guess. Disabled icons or sounds are not
 * played; the sound also respects the viewer's «Звук реакций» switch and the browser's autoplay rules.
 */
export function useAnswerReactions(look: EffectiveAppearance | undefined) {
  const [burst, setBurst] = useState<Burst | null>(null);
  const [soundOn, setSoundOn] = useSoundPreference();
  useEffect(() => {
    // Browsers allow audio after a user gesture: prepare the audio context on the first interaction.
    const unlock = () => unlockAudio();
    window.addEventListener('pointerdown', unlock);
    window.addEventListener('keydown', unlock);
    return () => {
      window.removeEventListener('pointerdown', unlock);
      window.removeEventListener('keydown', unlock);
    };
  }, []);
  const react = useCallback((correct: boolean) => {
    const r = look?.reactions;
    if (!r) return;
    const icon = correct ? r.correct_icon : r.incorrect_icon;
    if (icon.enabled && icon.icons.length) {
      const code = icon.icons[Math.floor(Math.random() * icon.icons.length)];
      setBurst({ key: Date.now(), icon: iconOf(correct, code), correct });
    }
    const sound = correct ? r.correct_sound : r.incorrect_sound;
    if (sound.enabled && soundOn) void playSound(correct ? 'correct' : 'incorrect', sound.sound);
  }, [look, soundOn]);
  const clear = useCallback(() => setBurst(null), []);
  const hasSound = !!look?.reactions && (look.reactions.correct_sound.enabled || look.reactions.incorrect_sound.enabled);
  return { react, burst, clear, hasSound, soundOn, setSoundOn };
}

/** The icon of the last reaction, floating up over the page (decorative: the answer itself shows the verdict). */
export function ReactionBurst({ burst, onDone }: { burst: Burst | null; onDone: () => void }) {
  useEffect(() => {
    if (!burst) return;
    const id = setTimeout(onDone, 1500); // also ends it when animations are reduced
    return () => clearTimeout(id);
  }, [burst, onDone]);
  if (!burst) return null;
  return (
    <div aria-hidden className="pointer-events-none fixed inset-x-0 bottom-24 z-50 flex justify-center" data-testid="answer-reaction" data-kind={burst.correct ? 'correct' : 'incorrect'}>
      <span key={burst.key} className="animate-reaction text-[72px] leading-none drop-shadow-lg motion-reduce:animate-none">{burst.icon}</span>
    </div>
  );
}

export function SoundToggle({ on, onChange }: { on: boolean; onChange: (v: boolean) => void }) {
  const t = useT();
  return (
    <button type="button" aria-pressed={on} onClick={() => onChange(!on)} data-testid="reaction-sound-toggle"
      className="inline-flex h-8 items-center gap-1.5 rounded-md border border-line-strong bg-surface px-3 text-sm font-medium text-ink-body transition-colors hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
      {on ? <Volume2 size={16} aria-hidden /> : <VolumeX size={16} aria-hidden />}
      {on ? t('marathons.look.soundOn') : t('marathons.look.soundOff')}
    </button>
  );
}
