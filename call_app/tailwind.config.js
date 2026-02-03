/**
 * Tailwind CSS Configuration for call_app
 *
 * PARCH DESIGN TOKENS
 * Source of truth: relay_server/static/parch-core.css
 * Keep color values in sync with parch-core.css
 *
 * @type {import('tailwindcss').Config}
 */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Parch color palette - sync with parch-core.css
        parch: {
          // Neutral Palette
          white: 'rgb(235, 235, 235)',
          'bright-white': 'rgb(245, 245, 245)',
          dark: 'rgb(25, 29, 37)',
          'second-dark': 'rgb(21, 24, 31)',
          black: 'rgb(19, 20, 24)',
          // Blue Palette (Interactive)
          'light-blue': 'rgb(66, 85, 119)',
          'second-blue': 'rgb(55, 71, 99)',
          'dark-blue': 'rgb(38, 44, 57)',
          'accent-blue': 'rgb(157, 179, 211)',
          // Neutral Accent
          gray: 'rgb(89, 96, 117)',
          // Status Colors
          'light-red': 'rgb(209, 102, 124)',
          'dark-red': 'rgb(152, 74, 90)',
          yellow: 'rgb(255, 250, 155)',
          green: 'rgb(119, 167, 77)',
        },
      },
      fontFamily: {
        serif: ['Georgia', 'Cambria', '"Times New Roman"', 'Times', 'serif'],
      },
      letterSpacing: {
        parch: '0.8px',
      },
    },
  },
  plugins: [],
}
