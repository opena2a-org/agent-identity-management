/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './pages/**/*.{ts,tsx}',
    './components/**/*.{ts,tsx}',
    './app/**/*.{ts,tsx}',
    './lib/**/*.{ts,tsx}',
  ],
  theme: {
    container: {
      center: true,
      padding: '2rem',
      screens: {
        '2xl': '1400px',
      },
    },
    extend: {
      fontFamily: {
        sans: ['var(--font-sans)'],
        mono: ['var(--font-mono)'],
      },
      colors: {
        // shadcn-compatible semantic colors; every value lives in app/globals.css
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        popover: {
          DEFAULT: 'hsl(var(--popover))',
          foreground: 'hsl(var(--popover-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
        // Glasshouse tokens
        brand: {
          DEFAULT: 'var(--brand)',
          hover: 'var(--brand-hover)',
          text: 'var(--brand-text)',
          soft: 'var(--brand-soft)',
          sky: 'var(--brand-sky)',
          indigo: 'var(--brand-indigo)',
        },
        success: {
          DEFAULT: 'var(--green)',
          text: 'var(--green-text)',
          fill: 'var(--green-fill)',
          border: 'var(--green-border)',
          foreground: '#ffffff',
        },
        warning: {
          DEFAULT: 'var(--amber)',
          text: 'var(--amber-text)',
          fill: 'var(--amber-fill)',
          border: 'var(--amber-border)',
          foreground: '#1d1d1f',
        },
        danger: {
          DEFAULT: 'var(--red)',
          text: 'var(--red-text)',
          fill: 'var(--red-fill)',
          border: 'var(--red-border)',
          foreground: '#ffffff',
        },
        ink: {
          DEFAULT: 'var(--text-primary)',
          body: 'var(--text-body)',
          secondary: 'var(--text-secondary)',
          tertiary: 'var(--text-tertiary)',
          inverse: 'var(--text-inverse)',
          'inverse-secondary': 'var(--text-inverse-secondary)',
          code: 'var(--text-code)',
        },
        glass: {
          DEFAULT: 'var(--glass-fill)',
          border: 'var(--glass-border)',
          chrome: 'var(--glass-chrome-fill)',
          'chrome-border': 'var(--glass-chrome-border)',
          inset: 'var(--surface-inset)',
          'inset-border': 'var(--surface-inset-border)',
          'inset-gray': 'var(--surface-inset-gray)',
          contrast: 'var(--surface-contrast)',
          'contrast-border': 'var(--surface-contrast-border)',
          code: 'var(--code-fill)',
        },
        segment: {
          track: 'var(--segment-track)',
          active: 'var(--segment-active)',
          text: 'var(--segment-inactive-text)',
        },
        nav: {
          active: 'var(--nav-active-fill)',
        },
        divider: 'var(--divider)',
        track: 'var(--track)',
        stroke: 'var(--stroke)',
        page: 'var(--bg-page-solid)',
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
        pill: '980px',
        chrome: '22px',
        card: '20px',
        'card-sm': '18px',
        panel: '16px',
        inset: '14px',
        'inset-sm': '12px',
        nav: '11px',
        avatar: '10px',
      },
      // Keys here must not collide with color names (colors.card / colors.accent would make
      // Tailwind emit a second .shadow-* rule for the ring color); hence panel/glow.
      boxShadow: {
        panel: 'var(--shadow-card)',
        chrome: 'var(--shadow-chrome)',
        modal: 'var(--shadow-modal)',
        glow: 'var(--brand-shadow)',
        segment: 'var(--segment-active-shadow)',
      },
      backdropBlur: {
        card: '20px',
        chrome: '24px',
      },
      backgroundImage: {
        logo: 'var(--gradient-logo)',
        bar: 'var(--gradient-bar)',
        'page-gradient': 'var(--bg-page)',
      },
      fontSize: {
        '2xs': ['10.5px', { lineHeight: '1.3' }],
      },
      keyframes: {
        'accordion-down': {
          from: { height: 0 },
          to: { height: 'var(--radix-accordion-content-height)' },
        },
        'accordion-up': {
          from: { height: 'var(--radix-accordion-content-height)' },
          to: { height: 0 },
        },
        'pulse-ring': {
          '0%': { boxShadow: '0 0 0 0 var(--brand-soft)' },
          '70%': { boxShadow: '0 0 0 8px transparent' },
          '100%': { boxShadow: '0 0 0 0 transparent' },
        },
      },
      animation: {
        'accordion-down': 'accordion-down 0.2s ease-out',
        'accordion-up': 'accordion-up 0.2s ease-out',
        'pulse-ring': 'pulse-ring 1.8s ease-out infinite',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
};
