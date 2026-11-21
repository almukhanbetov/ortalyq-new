'use client';

import { useRef, useState, type ReactNode } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ImagePlus, Trash2 } from 'lucide-react';
import { Button, useToast } from '@/components/ui';
import { marathonsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useT } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import type { ImageSlot } from '@/types/api';
import { IMAGE_MAX_BYTES, IMAGE_TYPES } from './reactions';

/** Re-reads everything that shows marathon pictures: header, cards, lessons, appearance. */
export function useImagesChanged(marathonId: string) {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: qk.classroom(marathonId) });
    qc.invalidateQueries({ queryKey: ['classrooms'] });
    qc.invalidateQueries({ queryKey: ['marathon', marathonId] });
  };
}

/**
 * Upload / replace / delete a cover or background picture. It takes effect at once (like an avatar), independently of
 * the «Сохранить» button of the dialog. The type and size are checked here for a quick answer; the server checks the
 * real content again (PNG, JPEG, WebP; up to 5 MB; 16–6000 px per side).
 */
export function ImageField({ marathonId, lessonId, slot, imageId, preview, uploadLabel, testId, onUploaded }: {
  marathonId: string; lessonId?: string; slot: ImageSlot; imageId: string | null | undefined;
  preview?: ReactNode; uploadLabel?: string; testId?: string; onUploaded?: () => void;
}) {
  const t = useT();
  const toast = useToast();
  const input = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);
  const changed = useImagesChanged(marathonId);
  const upload = useMutation({
    mutationFn: (f: File) => marathonsApi.uploadImage(marathonId, slot, f, lessonId),
    onSuccess: () => { setError(null); changed(); onUploaded?.(); toast(t('marathons.look.imageSaved'), 'success'); },
    onError: (e) => setError(errorMessage(e, t)),
  });
  const remove = useMutation({
    mutationFn: () => marathonsApi.removeImage(marathonId, slot, lessonId),
    onSuccess: () => { setError(null); changed(); toast(t('marathons.look.imageRemoved'), 'success'); },
    onError: (e) => setError(errorMessage(e, t)),
  });
  const pick = (f: File | undefined) => {
    if (!f) return;
    if (!IMAGE_TYPES.includes(f.type)) return setError(t('marathons.look.imageType'));
    if (f.size > IMAGE_MAX_BYTES) return setError(t('marathons.look.imageTooLarge'));
    upload.mutate(f);
  };
  const busy = upload.isPending || remove.isPending;

  return (
    <div data-testid={testId}>
      <div className="flex flex-wrap items-center gap-3">
        {preview}
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="secondary" icon={<ImagePlus size={16} />} loading={upload.isPending} disabled={busy}
            onClick={() => input.current?.click()}>
            {uploadLabel ?? (imageId ? t('marathons.look.replaceImage') : t('marathons.look.uploadImage'))}
          </Button>
          {imageId && (
            <Button size="sm" variant="ghost" icon={<Trash2 size={16} />} loading={remove.isPending} disabled={busy} onClick={() => remove.mutate()}>
              {t('marathons.look.removeImage')}
            </Button>
          )}
        </div>
      </div>
      <input ref={input} type="file" accept={IMAGE_TYPES.join(',')} hidden aria-hidden tabIndex={-1}
        onChange={(e) => { pick(e.target.files?.[0]); e.target.value = ''; }} />
      <p className="mt-1.5 text-xs text-ink-muted">{t('marathons.look.imageHint')}</p>
      {error && <p role="alert" className="mt-1 text-sm text-danger-text">{error}</p>}
    </div>
  );
}
