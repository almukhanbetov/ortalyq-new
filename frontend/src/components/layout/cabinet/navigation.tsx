import type { ReactNode } from 'react';
import { BookOpen, CalendarDays, CreditCard, Globe, GraduationCap, Hand, Settings, ShieldCheck, Users } from 'lucide-react';
import type { Role } from '@/types/api';
import { CLASSES_ROUTE, MARATHONS_ROUTE, MATERIALS_ROUTE } from '@/lib/routes';

export interface CabinetNavItem { href: string; key: string; icon: ReactNode; roles: Role[]; /** extra paths that also mark the item active */ also?: string[] }

const ICON = 20;

/** Single source of the cabinet menu. Unbuilt sections lead to the honest "in development" page. */
export const CABINET_NAV: CabinetNavItem[] = [
  { href: '/welcome', key: 'welcome', icon: <Hand size={ICON} />, roles: ['teacher', 'private_teacher', 'student', 'admin'] },
  { href: CLASSES_ROUTE, key: 'classes', icon: <GraduationCap size={ICON} />, roles: ['teacher', 'private_teacher', 'student'], also: [MARATHONS_ROUTE] },
  { href: '/students', key: 'students', icon: <Users size={ICON} />, roles: ['teacher'] },
  { href: MATERIALS_ROUTE, key: 'materials', icon: <BookOpen size={ICON} />, roles: ['teacher', 'private_teacher'] },
  { href: '/schedule', key: 'schedule', icon: <CalendarDays size={ICON} />, roles: ['teacher', 'private_teacher', 'student'] },
  { href: '/site', key: 'site', icon: <Globe size={ICON} />, roles: ['teacher', 'private_teacher'] },
  { href: '/pricing', key: 'pricing', icon: <CreditCard size={ICON} />, roles: ['teacher', 'private_teacher'] },
  { href: '/admin', key: 'admin', icon: <ShieldCheck size={ICON} />, roles: ['admin'] },
  { href: '/settings', key: 'settings', icon: <Settings size={ICON} />, roles: ['teacher', 'private_teacher', 'student', 'admin'] },
];

/** A divider is drawn after these items (groups: start / teaching / schedule and site / account). */
export const CABINET_DIVIDER_AFTER = new Set(['welcome', 'materials', 'site']);

export const SIDEBAR_EXPANDED = 288;
export const SIDEBAR_COLLAPSED = 64;

export function isActivePath(path: string, href: string, also: string[] = []) {
  return [href, ...also].some((h) => path === h || path.startsWith(h + '/'));
}
