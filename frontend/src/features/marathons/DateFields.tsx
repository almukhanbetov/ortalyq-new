'use client';

import { Field, Input, Notice, Select, Switch } from '@/components/ui';
import { useI18n } from '@/lib/i18n';
import { TIME_STEPS, formatMarathonMoment } from './marathonTime';

export interface DateDraft { on: boolean; date: string; time: string }

const DATE_TEXTS = {
  start: { label: 'marathons.startDate', hint: 'marathons.startDateHint', day: 'marathons.startDateDay', time: 'marathons.startDateTime',
    notice: 'marathons.startDateNotice', past: 'marathons.startDatePast' },
  end: { label: 'marathons.endDate', hint: 'marathons.endDateHint', day: 'marathons.endDateDay', time: 'marathons.endDateTime',
    notice: 'marathons.endDateNotice', past: 'marathons.endDatePast' },
  lesson: { label: 'marathons.lessonOpenDate', hint: 'marathons.lessonOpenDateHint', day: 'marathons.lessonOpenDateDay', time: 'marathons.lessonOpenDateTime',
    notice: 'marathons.lessonOpenDateNotice', past: 'marathons.lessonOpenDatePast' },
} as const;

/** "Отложенный старт" / "Дата завершения": switch + date + half-hour time in the marathon zone, with the moment spelled out. */
export function DateFields({ kind, value, onChange, timezone, iso, error }: {
  kind: keyof typeof DATE_TEXTS; value: DateDraft; onChange: (v: DateDraft) => void; timezone: string; iso: string | null; error?: string;
}) {
  const { t, locale } = useI18n();
  const tx = DATE_TEXTS[kind];
  const moment = iso ? formatMarathonMoment(iso, timezone, locale) : null;
  const zone = moment?.zone ?? formatMarathonMoment(new Date().toISOString(), timezone, locale).zone;
  const past = !!iso && Date.parse(iso) <= Date.now();
  const times = TIME_STEPS.includes(value.time) ? TIME_STEPS : [...TIME_STEPS, value.time].sort();
  return (
    <div className="flex flex-col gap-4">
      <Switch checked={value.on} onChange={(on) => onChange({ ...value, on })} label={t(tx.label)} hint={t(tx.hint)} />
      {value.on && (
        <>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field label={t(tx.day)} error={error}>
              {(p) => <Input {...p} type="date" value={value.date} required onChange={(e) => onChange({ ...value, date: e.target.value })} />}
            </Field>
            <Field label={`${t(tx.time)} (${zone})`}>
              {(p) => (
                <Select {...p} value={value.time} onChange={(e) => onChange({ ...value, time: e.target.value })}>
                  {times.map((tm) => <option key={tm} value={tm}>{tm} ({zone})</option>)}
                </Select>
              )}
            </Field>
          </div>
          {moment && (
            <Notice tone={past ? 'warning' : 'info'} className="text-sm">
              {past ? t(tx.past) : t(tx.notice, moment)}
            </Notice>
          )}
        </>
      )}
    </div>
  );
}

