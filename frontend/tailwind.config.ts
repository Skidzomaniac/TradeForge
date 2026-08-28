import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        surface: {
          primary: '#0a0a0a',
          secondary: '#111111',
          tertiary: '#1a1a1a',
          hover: '#222222',
        },
        accent: {
          green: '#00d1a0',
          red: '#ff4d6a',
          yellow: '#fbbf24',
          blue: '#3b82f6',
          purple: '#a78bfa',
        },
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'],
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
} satisfies Config
