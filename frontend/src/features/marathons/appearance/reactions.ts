import type { CorrectIcon, IncorrectIcon, MarathonBackground, Reactions } from '@/types/api';

/** Ready reaction icons. The server stores the codes; the emoji are drawn here. */
export const CORRECT_ICONS: Record<CorrectIcon, string> = {
  party_face: '🥳', party_popper: '🎉', muscle: '💪', fire: '🔥', brain: '🧠', check: '✅',
};
export const INCORRECT_ICONS: Record<IncorrectIcon, string> = {
  sad: '😥', grimace: '😬', see_no_evil: '🙈', thinking: '🤔', confused: '😕', cross: '❌',
};
export const SOUND_COUNT = 5;
/** Ready class backgrounds (CSS classes `class-bg-<preset>` in globals.css). */
export const BACKGROUND_PRESETS: Exclude<MarathonBackground, 'custom'>[] = ['plain', 'sunrise', 'waves', 'petals', 'leaves', 'autumn'];

/**
 * CSS classes of the backgrounds (globals.css). Written out in full: Tailwind keeps a component class only when it
 * finds its whole name in the sources, so a name built at runtime would be dropped from the build.
 */
export const BACKGROUND_CLASS: Record<Exclude<MarathonBackground, 'custom'>, string> = {
  plain: 'class-bg-plain', sunrise: 'class-bg-sunrise', waves: 'class-bg-waves', petals: 'class-bg-petals', leaves: 'class-bg-leaves', autumn: 'class-bg-autumn',
};

/** Client mirror of the server limits for appearance pictures (the server checks the content again). */
export const IMAGE_TYPES = ['image/png', 'image/jpeg', 'image/webp'];
export const IMAGE_MAX_BYTES = 5 * 1024 * 1024;

export const defaultReactions = (): Reactions => ({
  correct_icon: { enabled: true, icons: ['party_face', 'party_popper'] },
  incorrect_icon: { enabled: true, icons: ['sad', 'grimace'] },
  correct_sound: { enabled: true, sound: 1 },
  incorrect_sound: { enabled: true, sound: 1 },
});

export const iconOf = (correct: boolean, code: string) =>
  (correct ? CORRECT_ICONS[code as CorrectIcon] : INCORRECT_ICONS[code as IncorrectIcon]) ?? '';
