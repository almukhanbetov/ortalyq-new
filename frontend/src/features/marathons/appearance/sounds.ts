/**
 * Reaction sounds «Звук 1–5» are synthesised with the Web Audio API: no audio files, nothing to download or license.
 *
 * Browsers start audio only after the user has interacted with the page (autoplay policy). The context is created
 * and resumed on a user gesture (`unlockAudio`, called on pointer and key events of the lesson page and by the
 * «Прослушать» buttons). A reaction plays after the server has checked an answer, i.e. shortly after the click;
 * if the browser still keeps audio suspended, the sound is skipped silently and the icon is shown anyway.
 */

type Note = { f: number; to?: number; at: number; dur: number; wave: OscillatorType };

const C5 = 523.25, E5 = 659.25, G5 = 783.99, C6 = 1046.5, A5 = 880, D5 = 587.33;
const PATTERNS: Record<'correct' | 'incorrect', Note[][]> = {
  correct: [
    [{ f: C5, at: 0, dur: 0.12, wave: 'sine' }, { f: E5, at: 0.1, dur: 0.18, wave: 'sine' }],
    [{ f: E5, at: 0, dur: 0.1, wave: 'triangle' }, { f: G5, at: 0.09, dur: 0.1, wave: 'triangle' }, { f: C6, at: 0.18, dur: 0.22, wave: 'triangle' }],
    [{ f: A5, at: 0, dur: 0.45, wave: 'sine' }, { f: A5 * 2, at: 0, dur: 0.3, wave: 'sine' }],
    [{ f: G5, at: 0, dur: 0.08, wave: 'square' }, { f: C6, at: 0.09, dur: 0.16, wave: 'square' }],
    [{ f: D5, to: C6, at: 0, dur: 0.3, wave: 'sine' }],
  ],
  incorrect: [
    [{ f: 220, at: 0, dur: 0.16, wave: 'sine' }, { f: 174.6, at: 0.14, dur: 0.24, wave: 'sine' }],
    [{ f: 150, at: 0, dur: 0.28, wave: 'sawtooth' }],
    [{ f: 329.6, at: 0, dur: 0.14, wave: 'triangle' }, { f: 261.6, at: 0.13, dur: 0.22, wave: 'triangle' }],
    [{ f: 200, at: 0, dur: 0.09, wave: 'square' }, { f: 200, at: 0.15, dur: 0.09, wave: 'square' }],
    [{ f: 440, to: 160, at: 0, dur: 0.35, wave: 'sine' }],
  ],
};

let ctx: AudioContext | null = null;

function context(): AudioContext | null {
  if (typeof window === 'undefined') return null;
  const AC = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!AC) return null;
  if (!ctx) {
    try {
      ctx = new AC();
    } catch {
      return null;
    }
  }
  return ctx;
}

/** Call from a user gesture so later reactions may sound. */
export function unlockAudio() {
  const c = context();
  if (c && c.state === 'suspended') void c.resume().catch(() => {});
}

/** Plays «Звук n»; resolves to false when the browser does not allow audio now. */
export async function playSound(kind: 'correct' | 'incorrect', n: number): Promise<boolean> {
  const c = ctx ?? context();
  const notes = PATTERNS[kind][n - 1];
  if (!c || !notes) return false;
  if (c.state !== 'running') {
    try {
      await c.resume();
    } catch {
      return false;
    }
    if ((c.state as AudioContextState) !== 'running') return false; // re-read: resume() changes it
  }
  const t0 = c.currentTime + 0.01;
  for (const note of notes) {
    const osc = c.createOscillator();
    const gain = c.createGain();
    osc.type = note.wave;
    osc.frequency.setValueAtTime(note.f, t0 + note.at);
    if (note.to) osc.frequency.exponentialRampToValueAtTime(note.to, t0 + note.at + note.dur);
    const peak = note.wave === 'square' || note.wave === 'sawtooth' ? 0.05 : 0.14;
    gain.gain.setValueAtTime(0.0001, t0 + note.at);
    gain.gain.exponentialRampToValueAtTime(peak, t0 + note.at + 0.015);
    gain.gain.exponentialRampToValueAtTime(0.0001, t0 + note.at + note.dur);
    osc.connect(gain).connect(c.destination);
    osc.start(t0 + note.at);
    osc.stop(t0 + note.at + note.dur + 0.02);
  }
  return true;
}
