import type { Config } from 'tailwindcss';

// Design tokens mirror CSS variables in src/styles/tokens.css.
const v = (name: string) => `rgb(var(--${name}) / <alpha-value>)`;

const config: Config = {
  // hover: styles apply only on devices that can hover (no sticky highlight after a tap on phones).
  future: { hoverOnlyWhenSupported: true },
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: { DEFAULT: v('primary'), hover: v('primary-hover'), active: v('primary-active'), soft: v('primary-soft'), text: v('primary-text') },
        ink: { DEFAULT: v('ink'), body: v('ink-body'), muted: v('ink-muted'), soft: v('ink-soft'), faint: v('ink-faint') },
        page: v('page'),
        surface: { DEFAULT: v('surface'), alt: v('surface-alt'), hover: v('surface-hover'), muted: v('fill') },
        line: { DEFAULT: v('line'), strong: v('line-strong'), soft: v('line-soft'), shell: v('shell-line') },
        tooltip: v('tooltip'),
        success: { DEFAULT: v('success'), soft: v('success-soft'), text: v('success-text') },
        danger: { DEFAULT: v('danger'), soft: v('danger-soft'), text: v('danger-text') },
        warning: { DEFAULT: v('warning'), soft: v('warning-soft'), text: v('warning-text') },
        pink: { DEFAULT: v('pink'), soft: v('pink-soft'), text: v('pink-text') },
        info: { DEFAULT: v('info'), soft: v('info-soft'), text: v('info-text') },
      },
      fontFamily: { sans: ['Ubuntu', 'system-ui', 'sans-serif'] },
      fontSize: {
        xs: ['12px', '16px'],
        sm: ['13px', '18px'],
        base: ['14px', '20px'],
        md: ['16px', '22px'],
        lg: ['18px', '24px'],
        xl: ['22px', '28px'],
        '2xl': ['28px', '34px'],
        '3xl': ['32px', '40px'],
      },
      borderRadius: { sm: '6px', DEFAULT: '8px', md: '10px', lg: '12px', xl: '16px', '2xl': '20px' },
      boxShadow: {
        card: '0 1px 2px rgba(24,24,27,0.04), 0 2px 8px rgba(24,24,27,0.04)',
        pop: '0 8px 28px rgba(24,24,27,0.14)',
        modal: '0 16px 48px rgba(24,24,27,0.18)',
        focus: '0 0 0 3px rgba(26,198,255,0.35)',
      },
      spacing: { sidebar: '64px', topbar: '56px', cabinetbar: '64px' },
      keyframes: {
        'fade-in': { from: { opacity: '0' }, to: { opacity: '1' } },
        'pop-in': { from: { opacity: '0', transform: 'translateY(4px) scale(.98)' }, to: { opacity: '1', transform: 'none' } },
        // Reaction to a checked answer (marathons, stage 10): the icon jumps in, floats up and fades.
        reaction: {
          '0%': { opacity: '0', transform: 'translateY(12px) scale(.4)' },
          '18%': { opacity: '1', transform: 'translateY(0) scale(1.15)' },
          '30%': { transform: 'scale(1)' },
          '75%': { opacity: '1', transform: 'translateY(-16px) scale(1)' },
          '100%': { opacity: '0', transform: 'translateY(-36px) scale(.9)' },
        },
      },
      animation: { 'fade-in': 'fade-in .15s ease-out', 'pop-in': 'pop-in .16s ease-out', reaction: 'reaction 1.4s ease-out forwards' },
    },
  },
  plugins: [],
};
export default config;
