'use client';

import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AlertCircle, BookOpen, Plus, Trash2, Upload, X } from 'lucide-react';
import { Button, Input, Modal, Select, Tabs, Textarea, cn } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { marathonsApi, materialsApi, type CourseFields } from '@/lib/api/endpoints';
import { errorMessage, fieldErrors } from '@/lib/errors';
import { useI18n } from '@/lib/i18n';
import { qk } from '@/lib/queryKeys';
import type { Course } from '@/types/api';
import { COVER_COLORS, Cover } from '@/features/classrooms/Cover';

const TITLE_MAX = 200;
const DESCRIPTION_MAX = 5000;
const TAG_MAX = 40;
const TAGS_MAX = 20;
/** The same limits as the server (uploads.ImageMaxBytes, PNG/JPEG/WebP); the server checks the content again. */
const COVER_MAX_BYTES = 5 * 1024 * 1024;
const COVER_TYPES = ['image/png', 'image/jpeg', 'image/webp'];

type Tab = 'main' | 'tags';

/**
 * «Создать материал» / editing a material: tabs «Основные» (cover, title, description, language, «Разделы») and
 * «Теги». The values live in this form while switching tabs. A chosen cover is previewed at once and uploaded when
 * the material is saved; after a failed upload or save the form keeps everything for another try (an uploaded cover
 * is not uploaded twice).
 */
export function MaterialModal({ open, onClose, course, onSaved }: {
  open: boolean;
  onClose: () => void;
  /** The material being edited; without it the form creates a new one. */
  course?: Course;
  onSaved: (c: Course) => void;
}) {
  const { t, locale } = useI18n();
  const editing = !!course;
  const coverInput = useRef<HTMLInputElement>(null);
  const tagInput = useRef<HTMLInputElement>(null);
  const [tab, setTab] = useState<Tab>('main');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [language, setLanguage] = useState('');
  const [sections, setSections] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [tag, setTag] = useState('');
  const [tagError, setTagError] = useState<string | null>(null);
  const [color, setColor] = useState(COVER_COLORS[0]);
  // The cover: the saved picture, a newly chosen file (previewed from the computer) or none.
  const [coverId, setCoverId] = useState<string | null>(null);
  const [file, setFile] = useState<{ f: File; url: string; uploadedId: string | null } | null>(null);
  const [coverError, setCoverError] = useState<string | null>(null);

  // The form is filled when it opens (not when the material is re-read meanwhile: that would drop the edits).
  const source = useRef(course);
  source.current = course;
  useEffect(() => {
    if (!open) return;
    const course = source.current;
    setTab('main');
    setTitle(course?.title ?? '');
    setDescription(course?.description ?? '');
    setLanguage(course?.language ?? '');
    setSections(course?.sections_enabled ?? false); // a new material starts with «Разделы» off
    setTags(course?.tags ?? []);
    setTag(''); setTagError(null); setCoverError(null);
    setColor(course?.cover_color ?? COVER_COLORS[Math.floor(Math.random() * COVER_COLORS.length)]);
    setCoverId(course?.cover_image_id ?? null);
    setFile(null);
  }, [open, course?.id]);
  const fileUrl = file?.url;
  useEffect(() => () => { if (fileUrl) URL.revokeObjectURL(fileUrl); }, [fileUrl]);

  const langs = useQuery({ queryKey: qk.marathonLanguages, queryFn: marathonsApi.languages, staleTime: Infinity, enabled: open });
  const options = useMemo(
    () => [...(langs.data?.items ?? [])].sort((a, b) => t(`languages.${a}`).localeCompare(t(`languages.${b}`), locale)),
    [langs.data, t, locale],
  );

  const save = useMutation({
    mutationFn: async () => {
      let cover: string | undefined;
      if (file) {
        let id = file.uploadedId;
        if (!id) {
          try {
            id = (await materialsApi.uploadCover(file.f)).id;
          } catch (e) {
            throw new CoverError(e);
          }
          const uploaded = id;
          setFile((x) => (x && x.f === file.f ? { ...x, uploadedId: uploaded } : x));
        }
        cover = id;
      } else if (editing && course?.cover_image_id && !coverId) {
        cover = ''; // the picture was removed
      }
      const body: CourseFields = { title: title.trim(), description: description.trim(), language, sections_enabled: sections, tags, cover_color: color };
      if (cover !== undefined) body.cover_image_id = cover;
      return editing ? materialsApi.updateCourse(course!.id, body) : materialsApi.createCourse({ ...body, title: title.trim() });
    },
    onSuccess: (c) => onSaved(c),
  });

  const pickCover = (f: File | undefined) => {
    if (!f) return;
    setCoverError(null);
    if (!COVER_TYPES.includes(f.type)) return setCoverError(t('materialForm.coverType', { name: f.name }));
    if (f.size > COVER_MAX_BYTES) return setCoverError(t('materialForm.coverTooLarge', { name: f.name }));
    setFile({ f, url: URL.createObjectURL(f), uploadedId: null });
    save.reset();
  };
  const removeCover = () => { setFile(null); setCoverId(null); setCoverError(null); };

  const addTag = () => {
    const v = tag.trim().replace(/\s+/g, ' ');
    if (!v) return;
    if ([...v].length > TAG_MAX) return setTagError(t('materialForm.tagTooLong'));
    if (tags.includes(v)) { setTag(''); return setTagError(t('materialForm.tagExists', { tag: v })); }
    if (tags.length >= TAGS_MAX) return setTagError(t('materialForm.tagsLimit'));
    setTags((x) => [...x, v]);
    setTag(''); setTagError(null);
    tagInput.current?.focus();
  };

  const err = save.error;
  const errs = err instanceof ApiError ? fieldErrors(err) : {};
  const failure = err instanceof CoverError
    ? t('materialForm.coverFailed', { reason: coverReason(err.cause, t) })
    : err ? t('materialForm.saveFailed', { reason: errorMessage(err, t) }) : null;
  const submit = () => { if (title.trim() && !save.isPending) save.mutate(); };
  const preview = file?.url ?? (coverId ? materialsApi.coverUrl(coverId) : undefined);
  const tagsBadge = tags.length ? <span className="rounded-full bg-primary-soft px-1.5 text-xs font-medium text-primary-text">{tags.length}</span> : undefined;

  return (
    <Modal open={open} onClose={onClose} size="lg" bodyClassName="pt-0"
      title={<span className="inline-flex items-center gap-2.5"><span className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary-soft text-primary-text" aria-hidden><BookOpen size={20} /></span>{editing ? t('materialForm.editTitle') : t('materials.createMaterial')}</span>}
      footer={<>
        <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
        <Button onClick={submit} loading={save.isPending} disabled={!title.trim()}>{editing ? t('common.save') : t('materialForm.create')}</Button>
      </>}>
      <form onSubmit={(e) => { e.preventDefault(); submit(); }} data-testid="material-form">
        <Tabs<Tab> idPrefix="material-form" className="mb-1 border-b border-line" value={tab} onChange={setTab}
          items={[{ value: 'main', label: t('materialForm.main') }, { value: 'tags', label: t('materialForm.tags'), badge: tagsBadge }]} />

        <div role="tabpanel" aria-labelledby="material-form-tab-main" hidden={tab !== 'main'}>
          <Row label={t('materials.cover')} labelId="material-cover-label" group>
            {() => (
              <div className="flex flex-wrap items-center gap-3" aria-labelledby="material-cover-label" role="group">
                <Cover color={color} title={title || t('materialForm.name')} variant="tile" imageSrc={preview} imageId={preview ?? null}
                  className="h-12 w-12 shrink-0 rounded-lg [&_span]:text-lg" />
                <Button type="button" variant="secondary" size="sm" icon={<Upload size={16} />} onClick={() => coverInput.current?.click()}>{t('materialForm.coverUpload')}</Button>
                {preview && <Button type="button" variant="ghost" size="sm" icon={<Trash2 size={16} />} onClick={removeCover}>{t('materialForm.coverRemove')}</Button>}
                <input ref={coverInput} type="file" accept={COVER_TYPES.join(',')} hidden aria-hidden tabIndex={-1} data-testid="material-cover-input"
                  onChange={(e) => { pickCover(e.target.files?.[0]); e.target.value = ''; }} />
                <p className={cn('basis-full text-xs', coverError ? 'text-danger-text' : 'text-ink-muted')} role={coverError ? 'alert' : undefined} data-testid="material-cover-state">
                  {coverError ?? (file ? `${file.f.name} · ${t('materialForm.coverWillSave')}` : t('materialForm.coverHint'))}
                </p>
              </div>
            )}
          </Row>
          <Row label={t('materialForm.name')} required error={errs.title}>
            {(p) => <Input {...p} data-autofocus value={title} maxLength={TITLE_MAX} placeholder={t('materialForm.namePlaceholder')} onChange={(e) => setTitle(e.target.value)} />}
          </Row>
          <Row label={t('materialForm.description')} error={errs.description} top>
            {(p) => <Textarea {...p} rows={4} value={description} maxLength={DESCRIPTION_MAX} placeholder={t('materialForm.descriptionPlaceholder')} onChange={(e) => setDescription(e.target.value)} />}
          </Row>
          <Row label={t('materialForm.language')} error={errs.language}>
            {(p) => (
              <Select {...p} value={language} onChange={(e) => setLanguage(e.target.value)} className={cn(!language && 'text-ink-faint')}>
                <option value="">{t('materialForm.languagePlaceholder')}</option>
                {language && !options.includes(language) && <option value={language}>{t(`languages.${language}`)}</option>}
                {options.map((code) => <option key={code} value={code} className="text-ink">{t(`languages.${code}`)}</option>)}
              </Select>
            )}
          </Row>
          <Row label={t('materialForm.sections')} labelId="material-sections-label" last>
            {(p) => (
              <div className="flex items-start gap-3">
                <button id={p.id} type="button" role="switch" aria-checked={sections} aria-labelledby="material-sections-label" aria-describedby="material-sections-hint"
                  onClick={() => setSections((v) => !v)}
                  className={cn('relative mt-0.5 h-6 w-10 shrink-0 rounded-full transition-colors focus-visible:outline-none focus-visible:shadow-focus', sections ? 'bg-primary' : 'bg-line-strong')}>
                  <span className={cn('absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-all', sections ? 'left-[18px]' : 'left-0.5')} />
                </button>
                <p id="material-sections-hint" className="text-sm text-ink-muted">{sections ? t('materialForm.sectionsOn') : t('materialForm.sectionsOff')}</p>
              </div>
            )}
          </Row>
        </div>

        <div role="tabpanel" aria-labelledby="material-form-tab-tags" hidden={tab !== 'tags'} className="py-4">
          <p className="mb-3 text-sm text-ink-muted">{t('materialForm.tagsHint')}</p>
          <div className="flex gap-2">
            <label htmlFor="material-tag" className="sr-only">{t('materialForm.tagLabel')}</label>
            <Input id="material-tag" ref={tagInput} value={tag} maxLength={TAG_MAX + 10} placeholder={t('materialForm.tagPlaceholder')} invalid={!!tagError}
              onChange={(e) => { setTag(e.target.value); setTagError(null); }}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ',') { e.preventDefault(); addTag(); } }} />
            <Button type="button" variant="secondary" icon={<Plus size={16} />} onClick={addTag} disabled={!tag.trim()}>{t('materialForm.tagAdd')}</Button>
          </div>
          {(tagError || errs.tags) && <p className="mt-1.5 text-xs text-danger-text" role="alert">{tagError ?? errs.tags}</p>}
          {tags.length ? (
            <ul className="mt-4 flex flex-wrap gap-2" aria-label={t('materialForm.tags')} data-testid="material-tags">
              {tags.map((x) => (
                <li key={x} className="inline-flex h-8 items-center gap-1 rounded-full bg-primary-soft pl-3 pr-1 text-sm font-medium text-primary-text">
                  {x}
                  <button type="button" onClick={() => { setTags((all) => all.filter((y) => y !== x)); setTagError(null); }} aria-label={t('materialForm.tagRemove', { tag: x })}
                    className="flex h-6 w-6 items-center justify-center rounded-full hover:bg-primary/15 focus-visible:outline-none focus-visible:shadow-focus"><X size={14} /></button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="mt-4 text-sm text-ink-faint">{t('materialForm.tagsEmpty')}</p>
          )}
          <p className="mt-3 text-xs text-ink-muted">{t('materialForm.tagsCount', { n: tags.length, max: TAGS_MAX })}</p>
        </div>

        {failure && (
          <p role="alert" className="mt-3 flex items-start gap-2 rounded-lg bg-danger-soft px-3 py-2 text-sm text-danger-text" data-testid="material-form-error">
            <AlertCircle size={16} className="mt-0.5 shrink-0" />{failure}
          </p>
        )}
      </form>
    </Modal>
  );
}

/** A failed cover upload (shown apart from a failed save). */
class CoverError extends Error {
  constructor(public cause: unknown) { super('cover'); }
}

function coverReason(e: unknown, t: ReturnType<typeof useI18n>['t']) {
  if (e instanceof ApiError && e.code === 'image_type') return t('errors.image_type');
  if (e instanceof ApiError && e.code === 'file_too_large') return t('materialForm.coverTooLargeShort');
  return errorMessage(e, t);
}

/** One row of the form: the label on the left on a wide screen, above the field on a narrow one. */
function Row({ label, children, required, error, top, last, labelId, group }: {
  label: ReactNode; required?: boolean; error?: string; top?: boolean; last?: boolean; labelId?: string;
  /** The row holds several controls (a group named by the label), not one field the label points to. */
  group?: boolean;
  children: (p: { id: string; 'aria-describedby'?: string; invalid: boolean }) => ReactNode;
}) {
  const id = useId();
  const descId = error ? `${id}-err` : undefined;
  return (
    <div className={cn('grid gap-1.5 py-3.5 sm:grid-cols-[150px_minmax(0,1fr)] sm:gap-5', top ? 'sm:items-start' : 'sm:items-center', !last && 'border-b border-line-soft')}>
      <label id={labelId} htmlFor={group ? undefined : id} className={cn('text-sm font-medium text-ink-body sm:text-base', top && 'sm:pt-2')}>
        {label}{required && <span className="ml-0.5 text-danger" aria-hidden>*</span>}
      </label>
      <div className="min-w-0">
        {children({ id, 'aria-describedby': descId, invalid: !!error })}
        {error && <p id={descId} className="mt-1 text-xs text-danger-text" role="alert">{error}</p>}
      </div>
    </div>
  );
}
