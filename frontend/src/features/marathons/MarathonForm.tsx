'use client';

import { useMemo, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Field, Input, Select, cn } from '@/components/ui';
import { COVER_COLORS, Cover } from '@/features/classrooms/Cover';
import { marathonsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import type { MarathonDirection, MarathonSaveRequest } from '@/types/api';

export const emptyMarathon = (): MarathonSaveRequest => ({ name: '', cover_color: COVER_COLORS[0], direction: 'languages', target_language: null });

/** Client-side readiness mirrors the server rules; the server still validates everything. */
export function marathonReady(v: MarathonSaveRequest) {
  return v.name.trim() !== '' && (v.direction === 'other' || !!v.target_language);
}

/** Basic marathon fields: cover, name, direction, language. Used by "create" and by the settings dialog. */
export function MarathonForm({ value, onChange, errors, onSubmit, coverImageId, coverImage }: {
  value: MarathonSaveRequest;
  onChange: (v: MarathonSaveRequest) => void;
  errors: Record<string, string>;
  onSubmit: () => void;
  /** Settings of an existing marathon: the cover picture and its upload control (stage 10). */
  coverImageId?: string | null;
  coverImage?: ReactNode;
}) {
  const { t, locale } = useI18n();
  const langs = useQuery({ queryKey: qk.marathonLanguages, queryFn: marathonsApi.languages, staleTime: Infinity });
  const sorted = useMemo(
    () => [...(langs.data?.items ?? [])].sort((a, b) => t(`languages.${a}`).localeCompare(t(`languages.${b}`), locale)),
    [langs.data, t, locale],
  );
  const set = (patch: Partial<MarathonSaveRequest>) => onChange({ ...value, ...patch });
  const dirHint = value.direction === 'languages' ? t('marathons.directionLanguagesHint') : t('marathons.directionOtherHint');

  return (
    <form className="flex flex-col gap-4 pb-2" onSubmit={(e) => { e.preventDefault(); onSubmit(); }}>
      <fieldset>
        <legend className="mb-1.5 text-sm font-medium text-ink-body">{t('marathons.cover')}</legend>
        <div className="flex flex-wrap items-center gap-3">
          <Cover variant="tile" color={value.cover_color} imageId={coverImageId} title={value.name || t('marathons.title')} className="h-10 w-10 rounded-lg [&>span]:text-lg" />
          <div className="flex gap-2">
            {COVER_COLORS.map((c) => (
              <button key={c} type="button" onClick={() => set({ cover_color: c })} aria-label={t('marathons.coverColor', { color: c })} aria-pressed={c === value.cover_color}
                className={cn('h-8 w-8 rounded-full ring-offset-2 transition', c === value.cover_color && 'ring-2 ring-ink')} style={{ background: c }} />
            ))}
          </div>
        </div>
        {coverImage ? <div className="mt-3">{coverImage}</div> : <p className="mt-1.5 text-xs text-ink-muted">{t('marathons.coverUploadLater')}</p>}
        {errors.cover_color && <p role="alert" className="mt-1 text-sm text-danger-text">{errors.cover_color}</p>}
      </fieldset>
      <Field label={t('marathons.name')} error={errors.name} required>
        {(p) => <Input {...p} data-autofocus value={value.name} onChange={(e) => set({ name: e.target.value })} placeholder={t('marathons.namePlaceholder')} maxLength={120} />}
      </Field>
      <Field label={t('marathons.direction')} hint={dirHint} error={errors.direction} required>
        {(p) => (
          <Select {...p} value={value.direction} onChange={(e) => set({ direction: e.target.value as MarathonDirection })}>
            <option value="languages">{t('marathons.directionLanguages')}</option>
            <option value="other">{t('marathons.directionOther')}</option>
          </Select>
        )}
      </Field>
      {value.direction === 'languages' && (
        <Field label={t('marathons.language')} error={errors.target_language} required>
          {(p) => (
            <Select {...p} value={value.target_language ?? ''} onChange={(e) => set({ target_language: e.target.value || null })} disabled={!sorted.length}>
              <option value="">{t('marathons.languagePlaceholder')}</option>
              {sorted.map((code) => <option key={code} value={code}>{t(`languages.${code}`)}</option>)}
            </Select>
          )}
        </Field>
      )}
      <button type="submit" hidden aria-hidden tabIndex={-1} />
    </form>
  );
}
