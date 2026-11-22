import type { Role } from '@/types/api';

/** Roles that open the teacher cabinet: materials, exercises, preview, marathons. The server checks the same. */
export const TEACHING_ROLES: Role[] = ['teacher', 'private_teacher'];

export const isTeaching = (r: Role) => TEACHING_ROLES.includes(r);

/** Roles an administrator can set, in the order of the selects. */
export const ALL_ROLES: Role[] = ['admin', 'teacher', 'private_teacher', 'student'];
