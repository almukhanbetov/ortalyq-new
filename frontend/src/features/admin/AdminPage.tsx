'use client';

import { useState } from 'react';
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Link2Off, Plus, Search, UserCheck, UserX } from 'lucide-react';
import { Avatar, Badge, Button, EmptyState, Field, IconButton, Input, Modal, Pagination, QueryState, Select, Tabs, useToast } from '@/components/ui';
import { PageCard } from '@/components/layout/PageCard';
import { adminApi } from '@/lib/api/endpoints';
import { useT } from '@/lib/i18n';
import { errorMessage, fieldErrors } from '@/lib/errors';
import type { Role, User } from '@/types/api';

const LIMIT = 20;

export function AdminPage() {
  const t = useT();
  const [tab, setTab] = useState<'users' | 'assignments'>('users');
  return (
    <PageCard title={t('admin.title')}>
      <div className="mb-5 border-b border-line">
        <Tabs value={tab} onChange={setTab} items={[{ value: 'users', label: t('admin.users') }, { value: 'assignments', label: t('admin.assignments') }]} />
      </div>
      {tab === 'users' ? <UsersTab /> : <AssignmentsTab />}
    </PageCard>
  );
}

function UsersTab() {
  const t = useT();
  const qc = useQueryClient();
  const toast = useToast();
  const [q, setQ] = useState('');
  const [role, setRole] = useState('');
  const [offset, setOffset] = useState(0);
  const [creating, setCreating] = useState(false);
  const [pwFor, setPwFor] = useState<User | null>(null);
  const users = useQuery({ queryKey: ['admin-users', q, role, offset], queryFn: () => adminApi.users({ q, role, limit: LIMIT, offset }), placeholderData: keepPreviousData });
  const update = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Parameters<typeof adminApi.updateUser>[1] }) => adminApi.updateUser(id, body),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin-users'] }); setPwFor(null); toast(t('common.save') + ' ✓', 'success'); },
    onError: (e) => toast(errorMessage(e, t), 'error'),
  });

  return (
    <>
      <div className="mb-4 flex flex-wrap gap-3">
        <div className="relative min-w-[220px] flex-1">
          <Search size={18} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-faint" aria-hidden />
          <Input className="pl-10" value={q} placeholder={t('admin.searchUsers')} aria-label={t('admin.searchUsers')} onChange={(e) => { setQ(e.target.value); setOffset(0); }} />
        </div>
        <Select className="w-auto min-w-[180px]" value={role} aria-label={t('admin.role')} onChange={(e) => { setRole(e.target.value); setOffset(0); }}>
          <option value="">{t('admin.allRoles')}</option>
          {(['admin', 'teacher', 'student'] as Role[]).map((r) => <option key={r} value={r}>{t(`roles.${r}`)}</option>)}
        </Select>
        <Button icon={<Plus size={18} />} onClick={() => setCreating(true)}>{t('admin.createUser')}</Button>
      </div>
      <QueryState query={users}>
        {(data) => data.items.length === 0 ? <EmptyState title={t('admin.noUsers')} /> : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full min-w-[640px] text-left text-base">
                <thead className="border-b border-line text-sm text-ink-muted">
                  <tr>
                    <th className="py-2 font-medium">{t('admin.fullName')}</th>
                    <th className="py-2 font-medium">{t('admin.role')}</th>
                    <th className="py-2 font-medium">{t('admin.status')}</th>
                    <th className="py-2" />
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((u) => (
                    <tr key={u.id} className="border-b border-line last:border-0">
                      <td className="py-3">
                        <div className="flex items-center gap-3">
                          <Avatar name={u.full_name} color={u.avatar_color} size={32} />
                          <div className="min-w-0"><p className="truncate font-medium text-ink">{u.full_name}</p><p className="truncate text-sm text-ink-muted">{u.email}</p></div>
                        </div>
                      </td>
                      <td className="py-3">
                        <Select className="h-9 w-auto text-sm" value={u.role} aria-label={t('admin.role')} onChange={(e) => update.mutate({ id: u.id, body: { role: e.target.value } })}>
                          {(['admin', 'teacher', 'student'] as Role[]).map((r) => <option key={r} value={r}>{t(`roles.${r}`)}</option>)}
                        </Select>
                      </td>
                      <td className="py-3">{u.disabled_at ? <Badge tone="danger">{t('admin.disabled')}</Badge> : <Badge tone="success">{t('admin.active')}</Badge>}</td>
                      <td className="py-3 text-right">
                        <IconButton label={t('admin.resetPassword')} onClick={() => setPwFor(u)}><KeyRound size={16} /></IconButton>
                        <IconButton label={u.disabled_at ? t('admin.enable') : t('admin.disable')} tone={u.disabled_at ? 'default' : 'danger'}
                          onClick={() => update.mutate({ id: u.id, body: { disabled: !u.disabled_at } })}>{u.disabled_at ? <UserCheck size={16} /> : <UserX size={16} />}</IconButton>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} />
          </>
        )}
      </QueryState>
      <CreateUserModal open={creating} onClose={() => setCreating(false)} />
      <PasswordModal user={pwFor} onClose={() => setPwFor(null)} loading={update.isPending} error={fieldErrors(update.error).password}
        onSubmit={(password) => pwFor && update.mutate({ id: pwFor.id, body: { password } })} />
    </>
  );
}

function CreateUserModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const [form, setForm] = useState({ email: '', full_name: '', role: 'student', password: '' });
  const create = useMutation({
    mutationFn: () => adminApi.createUser(form),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['admin-users'] }); setForm({ email: '', full_name: '', role: 'student', password: '' }); onClose(); },
  });
  const errs = fieldErrors(create.error);
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));
  return (
    <Modal open={open} onClose={onClose} title={t('admin.createUser')}
      footer={<><Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button><Button loading={create.isPending} onClick={() => create.mutate()}>{t('common.create')}</Button></>}>
      <form className="grid gap-4 pb-2" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <Field label={t('admin.fullName')} error={errs.full_name} required>{(p) => <Input {...p} data-autofocus value={form.full_name} onChange={set('full_name')} />}</Field>
        <Field label={t('admin.email')} error={errs.email} required>{(p) => <Input {...p} type="email" value={form.email} onChange={set('email')} />}</Field>
        <Field label={t('admin.role')} error={errs.role}>
          {(p) => <Select {...p} value={form.role} onChange={set('role')}>{(['student', 'teacher', 'admin'] as Role[]).map((r) => <option key={r} value={r}>{t(`roles.${r}`)}</option>)}</Select>}
        </Field>
        <Field label={t('admin.password')} hint={t('admin.passwordHint')} error={errs.password} required>{(p) => <Input {...p} type="password" autoComplete="new-password" value={form.password} onChange={set('password')} />}</Field>
        {create.error && !Object.keys(errs).length && <p role="alert" className="text-sm text-danger-text">{errorMessage(create.error, t)}</p>}
      </form>
    </Modal>
  );
}

function PasswordModal({ user, onClose, onSubmit, loading, error }: { user: User | null; onClose: () => void; onSubmit: (pw: string) => void; loading: boolean; error?: string }) {
  const t = useT();
  const [pw, setPw] = useState('');
  return (
    <Modal open={!!user} onClose={onClose} title={`${t('admin.resetPassword')}: ${user?.full_name ?? ''}`} size="sm"
      footer={<><Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button><Button loading={loading} disabled={pw.length < 8} onClick={() => { onSubmit(pw); setPw(''); }}>{t('common.save')}</Button></>}>
      <Field label={t('admin.newPassword')} hint={t('admin.passwordHint')} error={error}>
        {(p) => <Input {...p} data-autofocus type="password" autoComplete="new-password" value={pw} onChange={(e) => setPw(e.target.value)} />}
      </Field>
    </Modal>
  );
}

function AssignmentsTab() {
  const t = useT();
  const qc = useQueryClient();
  const toast = useToast();
  const [teacher, setTeacher] = useState('');
  const [student, setStudent] = useState('');
  const [offset, setOffset] = useState(0);
  const teachers = useQuery({ queryKey: ['admin-users', '', 'teacher', 'all'], queryFn: () => adminApi.users({ role: 'teacher', limit: 200 }) });
  const students = useQuery({ queryKey: ['admin-users', '', 'student', 'all'], queryFn: () => adminApi.users({ role: 'student', limit: 200 }) });
  const list = useQuery({ queryKey: ['admin-assignments', offset], queryFn: () => adminApi.assignments({ limit: LIMIT, offset }), placeholderData: keepPreviousData });
  const refresh = () => qc.invalidateQueries({ queryKey: ['admin-assignments'] });
  const assign = useMutation({ mutationFn: () => adminApi.assign(teacher, student), onSuccess: () => { refresh(); setStudent(''); }, onError: (e) => toast(errorMessage(e, t), 'error') });
  const unassign = useMutation({ mutationFn: (a: { t: string; s: string }) => adminApi.unassign(a.t, a.s), onSuccess: refresh, onError: (e) => toast(errorMessage(e, t), 'error') });
  return (
    <>
      <div className="mb-6 grid gap-3 rounded-xl bg-surface-alt p-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
        <Field label={t('admin.teacher')}>
          {(p) => <Select {...p} value={teacher} onChange={(e) => setTeacher(e.target.value)}><option value="">{t('admin.select')}</option>{teachers.data?.items.map((u) => <option key={u.id} value={u.id}>{u.full_name}</option>)}</Select>}
        </Field>
        <Field label={t('admin.student')}>
          {(p) => <Select {...p} value={student} onChange={(e) => setStudent(e.target.value)}><option value="">{t('admin.select')}</option>{students.data?.items.map((u) => <option key={u.id} value={u.id}>{u.full_name}</option>)}</Select>}
        </Field>
        <Button disabled={!teacher || !student} loading={assign.isPending} onClick={() => assign.mutate()}>{t('admin.assign')}</Button>
      </div>
      <QueryState query={list}>
        {(data) => data.items.length === 0 ? <EmptyState title={t('admin.noAssignments')} /> : (
          <>
            <ul className="flex flex-col divide-y divide-line">
              {data.items.map((a) => (
                <li key={a.teacher.id + a.student.id} className="flex flex-wrap items-center gap-3 py-3">
                  <Avatar name={a.teacher.full_name} color={a.teacher.avatar_color} size={28} />
                  <span className="font-medium text-ink">{a.teacher.full_name}</span>
                  <span className="text-ink-muted">→</span>
                  <Avatar name={a.student.full_name} color={a.student.avatar_color} size={28} />
                  <span className="flex-1 text-ink">{a.student.full_name}</span>
                  <IconButton label={t('admin.unassign')} tone="danger" onClick={() => unassign.mutate({ t: a.teacher.id, s: a.student.id })}><Link2Off size={16} /></IconButton>
                </li>
              ))}
            </ul>
            <Pagination total={data.total} limit={LIMIT} offset={offset} onChange={setOffset} />
          </>
        )}
      </QueryState>
    </>
  );
}
