'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { MonitorPlay, Presentation, X } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { Button, EmptyState, Modal, Notice, QueryState, RadioCard, Switch, useToast } from '@/components/ui';
import { ApiError } from '@/lib/api/client';
import { classBoardApi, classroomsApi } from '@/lib/api/endpoints';
import { qk } from '@/lib/queryKeys';
import { useI18n } from '@/lib/i18n';
import { errorMessage } from '@/lib/errors';
import { boardUrl, classUrl } from '@/lib/routes';

const newRequestId = () => (typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID()
  : '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, (c) => (Number(c) ^ (Math.random() * 16) >> (Number(c) / 4)).toString(16)));

/** «Открыть в классе»: the teacher picks one of their classes; the board appears in that lesson. */
export function OpenInClassButton({ boardId }: { boardId: string }) {
  const { t } = useI18n();
  const toast = useToast();
  const qc = useQueryClient();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [picked, setPicked] = useState<string | null>(null);
  const [copy, setCopy] = useState(false);
  // One id per opening of the window: a resent or double-clicked request returns the same copy.
  const [requestId, setRequestId] = useState('');
  const classes = useQuery({ queryKey: ['classrooms', 'board-targets'], queryFn: () => classroomsApi.list({ limit: 100 }), enabled: open });
  const mine = (classes.data?.items ?? []).filter((c) => c.my_role === 'teacher' && c.lesson_type !== 'marathon');
  const openM = useMutation({
    mutationFn: (cid: string) => (copy ? classBoardApi.openCopy(cid, boardId, requestId) : classBoardApi.open(cid, boardId)),
    retry: (n, e) => copy && !(e instanceof ApiError && e.status < 500) && n < 2,
    onSuccess: (v, cid) => {
      const name = mine.find((c) => c.id === cid)?.name ?? '';
      qc.invalidateQueries({ queryKey: qk.board(boardId) });
      qc.invalidateQueries({ queryKey: qk.classBoard(cid) });
      qc.invalidateQueries({ queryKey: ['boards'] });
      setOpen(false);
      if (v.copy) {
        // The lesson now works on the copy: go to it (it shows where it is open, with «Перейти в класс»).
        toast(t('boards.copyOpenedInClass', { title: v.copy.title, name }), 'success');
        router.push(boardUrl(v.copy.id));
      } else {
        toast(t('boards.openedInClass', { name }), 'success');
      }
    },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
  return (
    <>
      <Button size="sm" variant="soft" icon={<Presentation size={16} />}
        onClick={() => { setPicked(null); setCopy(false); setRequestId(newRequestId()); setOpen(true); }}>{t('boards.openInClass')}</Button>
      <Modal open={open} onClose={() => setOpen(false)} title={t('boards.openInClass')} size="md"
        footer={<>
          <Button variant="secondary" onClick={() => setOpen(false)}>{t('common.cancel')}</Button>
          <Button disabled={!picked} loading={openM.isPending} onClick={() => picked && openM.mutate(picked)}>{t('boards.openInClassConfirm')}</Button>
        </>}>
        <p className="mb-4 text-sm text-ink-muted">{t('boards.openInClassHint')}</p>
        <QueryState query={classes}>
          {() => mine.length === 0 ? <EmptyState title={t('boards.noClasses')} /> : (
            <fieldset className="flex flex-col gap-2 pb-2">
              <legend className="sr-only">{t('boards.chooseClass')}</legend>
              {mine.map((c) => (
                <RadioCard key={c.id} name="board-class" value={c.id} checked={picked === c.id} onChange={() => setPicked(c.id)}
                  title={c.name} hint={c.members.filter((m) => m.role === 'student').map((m) => m.full_name).join(', ')} />
              ))}
            </fieldset>
          )}
        </QueryState>
        <div className="mt-2 rounded-xl border border-line p-3">
          <Switch checked={copy} onChange={setCopy} label={t('boards.copyForLesson')} hint={t('boards.copyForLessonHint')} />
        </div>
      </Modal>
    </>
  );
}

/** Where the board is open now, with «Перейти в класс» and «Закрыть доску в классе». */
export function OpenInClassBanner({ boardId, classes }: { boardId: string; classes: { classroom_id: string; name: string }[] }) {
  const { t } = useI18n();
  const close = useCloseInClass(boardId);
  return (
    <div className="mt-3 flex flex-col gap-2">
      {classes.map((c) => (
        <Notice key={c.classroom_id} tone="info" icon={<MonitorPlay size={18} />}
          action={<div className="flex flex-wrap gap-2">
            <Link href={classUrl({ id: c.classroom_id, lesson_type: 'individual' })} className="inline-flex h-8 items-center rounded-md px-3 text-sm font-medium text-primary-text hover:underline">{t('boards.goToClass')}</Link>
            <Button size="sm" variant="secondary" icon={<X size={16} />} loading={close.isPending && close.variables === c.classroom_id} onClick={() => close.mutate(c.classroom_id)}>{t('boards.closeInClass')}</Button>
          </div>}>
          <span className="flex-1 text-sm">{t('boards.openInClassNotice', { name: c.name })}</span>
        </Notice>
      ))}
    </div>
  );
}

/** «Закрыть доску в классе»: the lesson access ends; the board stays in «Материалы». */
export function useCloseInClass(boardId?: string) {
  const { t } = useI18n();
  const toast = useToast();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (cid: string) => classBoardApi.close(cid),
    onSuccess: (_, cid) => {
      if (boardId) qc.invalidateQueries({ queryKey: qk.board(boardId) });
      qc.invalidateQueries({ queryKey: qk.classBoard(cid) });
      toast(t('boards.closedInClass'), 'success');
    },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });
}
