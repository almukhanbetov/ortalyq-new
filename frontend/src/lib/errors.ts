import { ApiError } from '@/lib/api/client';
import type { TFunction } from '@/lib/i18n';

/** Human-readable message for any error, localized where we have a key. */
export function errorMessage(err: unknown, t: TFunction): string {
  if (err instanceof ApiError) {
    if (err.isNetwork) return t('common.networkError');
    // A refused sign-in is not an expired session: say what really happened.
    if (err.status === 401 && err.code === 'invalid_credentials') return t('errors.invalid_credentials');
    if (err.status === 401) return t('errors.unauthorized');
    if (err.status === 403 && err.code === 'forbidden') return t('errors.forbidden');
    if (err.code === 'marathon_closed') return t('errors.marathon_closed');
    if (err.code === 'marathon_deactivated') return t('errors.marathon_deactivated');
    if (err.code === 'marathon_auto_deactivated') return t('errors.marathon_auto_deactivated');
    if (err.code === 'marathon_not_started') return t('errors.marathon_not_started');
    if (err.code === 'marathon_lesson_locked') return t('errors.marathon_lesson_locked');
    if (err.code === 'lesson_dates_set') return t('errors.lesson_dates_set');
    if (err.code === 'settings_changed') return t('errors.settings_changed');
    if (err.code === 'image_type' || err.code === 'image_size' || err.code === 'file_too_large' || err.code === 'pdf_type') return t(`errors.${err.code}`);
    if (err.status === 404) return t('errors.not_found');
    const fieldMsgs = Object.values(err.fields);
    if (fieldMsgs.length) return fieldMsgs.join('. ');
    return err.message || t('common.unknownError');
  }
  return t('common.unknownError');
}

export function fieldErrors(err: unknown): Record<string, string> {
  return err instanceof ApiError ? err.fields : {};
}
