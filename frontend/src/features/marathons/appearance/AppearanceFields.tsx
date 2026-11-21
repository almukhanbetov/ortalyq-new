'use client';

import { useEffect, useRef } from 'react';
import { Check, Play } from 'lucide-react';
import { Select, Switch, cn } from '@/components/ui';
import { attachmentUrl } from '@/lib/api/client';
import { useT } from '@/lib/i18n';
import type { Appearance, AppearanceRequest, CorrectIcon, IconReaction, IncorrectIcon, MarathonBackground, Reactions, SoundReaction } from '@/types/api';
import { BACKGROUND_CLASS, BACKGROUND_PRESETS, CORRECT_ICONS, INCORRECT_ICONS, SOUND_COUNT } from './reactions';
import { playSound, unlockAudio } from './sounds';
import { ImageField } from './ImageField';

/** Editable part of an appearance (marathon or lesson): reactions and background. */
export type AppearanceDraft = Pick<AppearanceRequest, 'reactions_enabled' | 'reactions' | 'background_enabled' | 'background'>;

export const draftOf = (a: Appearance): AppearanceDraft => ({
  reactions_enabled: a.reactions_enabled, reactions: a.reactions, background_enabled: a.background_enabled, background: a.background,
});

/** Client mirror of the server rules: an enabled icon needs at least one icon; custom needs a picture. */
export function draftValid(d: AppearanceDraft, hasImage: boolean) {
  const r = d.reactions;
  return (!r.correct_icon.enabled || r.correct_icon.icons.length > 0) && (!r.incorrect_icon.enabled || r.incorrect_icon.icons.length > 0)
    && (d.background !== 'custom' || hasImage);
}

/**
 * «Реакции на ответы» and «Фон в классе». Used by the «Оформление» tab of the marathon settings and by the
 * appearance tab of a lesson. Own icons and sounds and recording from the microphone are not offered (stage limits).
 */
export function AppearanceFields({ value, onChange, marathonId, lessonId, backgroundImageId, idPrefix }: {
  value: AppearanceDraft; onChange: (v: AppearanceDraft) => void;
  marathonId: string; lessonId?: string; backgroundImageId: string | null; idPrefix: string;
}) {
  const t = useT();
  const set = (patch: Partial<AppearanceDraft>) => onChange({ ...value, ...patch });
  const setR = <K extends keyof Reactions>(k: K, v: Reactions[K]) => set({ reactions: { ...value.reactions, [k]: v } });

  return (
    <div className="flex flex-col divide-y divide-line-soft" data-testid={`${idPrefix}-appearance`}>
      <div className="flex flex-col gap-4 py-4">
        <Switch checked={value.reactions_enabled} onChange={(v) => set({ reactions_enabled: v })}
          label={t('marathons.look.reactions')} hint={t('marathons.look.reactionsHint')} />
        {value.reactions_enabled && (
          <div className="flex flex-col gap-5 rounded-xl bg-surface-alt p-4" data-testid={`${idPrefix}-reactions`}>
            <IconPicker label={t('marathons.look.correctIcon')} value={value.reactions.correct_icon} icons={CORRECT_ICONS}
              onChange={(v) => setR('correct_icon', v as IconReaction<CorrectIcon>)} testId={`${idPrefix}-correct-icons`} />
            <IconPicker label={t('marathons.look.incorrectIcon')} value={value.reactions.incorrect_icon} icons={INCORRECT_ICONS}
              onChange={(v) => setR('incorrect_icon', v as IconReaction<IncorrectIcon>)} testId={`${idPrefix}-incorrect-icons`} />
            <SoundPicker label={t('marathons.look.correctSound')} kind="correct" value={value.reactions.correct_sound}
              onChange={(v) => setR('correct_sound', v)} testId={`${idPrefix}-correct-sound`} />
            <SoundPicker label={t('marathons.look.incorrectSound')} kind="incorrect" value={value.reactions.incorrect_sound}
              onChange={(v) => setR('incorrect_sound', v)} testId={`${idPrefix}-incorrect-sound`} />
            <p className="text-xs text-ink-muted">{t('marathons.look.ownReactionsUnavailable')}</p>
          </div>
        )}
      </div>
      <div className="flex flex-col gap-4 py-4">
        <Switch checked={value.background_enabled} onChange={(v) => set({ background_enabled: v })}
          label={t('marathons.look.background')} hint={t('marathons.look.backgroundHint')} />
        {value.background_enabled && (
          <BackgroundPicker value={value.background} onChange={(b) => set({ background: b })} marathonId={marathonId} lessonId={lessonId}
            imageId={backgroundImageId} name={`${idPrefix}-bg`} />
        )}
      </div>
    </div>
  );
}

function IconPicker<T extends string>({ label, value, icons, onChange, testId }: {
  label: string; value: IconReaction<T>; icons: Record<T, string>; onChange: (v: IconReaction<T>) => void; testId: string;
}) {
  const t = useT();
  const toggle = (code: T) => onChange({ ...value, icons: value.icons.includes(code) ? value.icons.filter((x) => x !== code) : [...value.icons, code] });
  return (
    <div className="flex flex-col gap-2" data-testid={testId}>
      <Switch checked={value.enabled} onChange={(v) => onChange({ ...value, enabled: v })} label={label} />
      {value.enabled && (
        <>
          <div role="group" aria-label={label} className="flex flex-wrap gap-2">
            {(Object.keys(icons) as T[]).map((code) => {
              const on = value.icons.includes(code);
              return (
                <button key={code} type="button" aria-pressed={on} onClick={() => toggle(code)} data-icon={code}
                  aria-label={t(`marathons.look.icons.${code}`)} title={t(`marathons.look.icons.${code}`)}
                  className={cn('relative flex h-11 w-11 items-center justify-center rounded-lg border-2 text-[22px] transition-colors focus-visible:shadow-focus focus-visible:outline-none',
                    on ? 'border-primary bg-primary-soft' : 'border-line bg-surface hover:border-line-strong')}>
                  <span aria-hidden>{icons[code]}</span>
                  {on && <Check size={12} aria-hidden className="absolute right-0.5 top-0.5 text-primary-text" />}
                </button>
              );
            })}
          </div>
          {value.icons.length === 0 && <p role="alert" className="text-sm text-danger-text">{t('marathons.look.pickIcon')}</p>}
        </>
      )}
    </div>
  );
}

function SoundPicker({ label, kind, value, onChange, testId }: {
  label: string; kind: 'correct' | 'incorrect'; value: SoundReaction; onChange: (v: SoundReaction) => void; testId: string;
}) {
  const t = useT();
  return (
    <div className="flex flex-col gap-2" data-testid={testId}>
      <Switch checked={value.enabled} onChange={(v) => onChange({ ...value, enabled: v })} label={label} />
      {value.enabled && (
        <div className="flex items-center gap-2">
          <Select aria-label={label} value={String(value.sound)} onChange={(e) => onChange({ ...value, sound: Number(e.target.value) })} className="w-40">
            {Array.from({ length: SOUND_COUNT }, (_, i) => <option key={i + 1} value={i + 1}>{t('marathons.look.soundN', { n: i + 1 })}</option>)}
          </Select>
          <button type="button" onClick={() => { unlockAudio(); void playSound(kind, value.sound); }}
            aria-label={t('marathons.look.listen', { n: value.sound })} title={t('marathons.look.listen', { n: value.sound })}
            className="flex h-10 w-10 items-center justify-center rounded-md border border-line-strong bg-surface text-ink-body transition-colors hover:bg-surface-hover focus-visible:shadow-focus focus-visible:outline-none">
            <Play size={16} aria-hidden />
          </button>
        </div>
      )}
    </div>
  );
}

function BackgroundPicker({ value, onChange, marathonId, lessonId, imageId, name }: {
  value: MarathonBackground; onChange: (v: MarathonBackground) => void; marathonId: string; lessonId?: string; imageId: string | null; name: string;
}) {
  const t = useT();
  const tile = (selected: boolean) => cn('relative block aspect-[16/10] w-full overflow-hidden rounded-lg border-2 transition-colors focus-within:shadow-focus',
    selected ? 'border-primary' : 'border-line hover:border-line-strong');
  // The custom picture was deleted (here or in another tab): the server fell back to the plain background.
  // Only a transition picture → none counts: right after an upload the new picture is still being re-read.
  const prevImage = useRef(imageId);
  useEffect(() => {
    if (prevImage.current && !imageId && value === 'custom') onChange('plain');
    prevImage.current = imageId;
  }, [value, imageId, onChange]);
  const mark = <span className="absolute left-1.5 top-1.5 flex h-5 w-5 items-center justify-center rounded-full bg-primary text-white"><Check size={12} aria-hidden /></span>;
  return (
    <div className="flex flex-col gap-3">
      <div role="radiogroup" aria-label={t('marathons.look.background')} className="grid grid-cols-2 gap-3 sm:grid-cols-3" data-testid={`${name}-options`}>
        {BACKGROUND_PRESETS.map((b) => (
          <label key={b} className={tile(value === b)} title={t(`marathons.look.backgrounds.${b}`)}>
            <input type="radio" name={name} value={b} checked={value === b} onChange={() => onChange(b)} className="sr-only" />
            <span aria-hidden className={cn('absolute inset-0', BACKGROUND_CLASS[b])} />
            {value === b && mark}
            <span className="absolute inset-x-0 bottom-0 bg-surface/85 px-2 py-0.5 text-xs font-medium text-ink-body">{t(`marathons.look.backgrounds.${b}`)}</span>
          </label>
        ))}
        {imageId && (
          <label className={tile(value === 'custom')} title={t('marathons.look.backgrounds.custom')}>
            <input type="radio" name={name} value="custom" checked={value === 'custom'} onChange={() => onChange('custom')} className="sr-only" />
            {/* eslint-disable-next-line @next/next/no-img-element -- private attachment of the marathon */}
            <img src={attachmentUrl(imageId, true)} alt="" className="absolute inset-0 h-full w-full object-cover" />
            {value === 'custom' && mark}
            <span className="absolute inset-x-0 bottom-0 bg-surface/85 px-2 py-0.5 text-xs font-medium text-ink-body">{t('marathons.look.backgrounds.custom')}</span>
          </label>
        )}
      </div>
      <ImageField marathonId={marathonId} lessonId={lessonId} slot="background" imageId={imageId} testId={`${name}-image`} onUploaded={() => onChange('custom')}
        uploadLabel={imageId ? t('marathons.look.replaceBackground') : t('marathons.look.uploadBackground')} />
    </div>
  );
}
