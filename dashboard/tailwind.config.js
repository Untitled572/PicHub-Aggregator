/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        morandi: {
          bg: '#F4F6F8',
          card: '#FFFFFF',
          sidebar: '#EBEFF3',
          hover: '#E1E7EE',
          border: '#D2DBE4',
          borderSoft: '#E1E7EE',
          text: '#22303C',
          muted: '#687887',
          light: '#98A8B7',

          // Cold Slate Blue (冷灰蓝) Primary
          blue: '#4A6B82',
          'blue-light': '#EDF3F8',
          'blue-dark': '#344E61',

          sage: '#4A6B82',
          'sage-light': '#EDF3F8',
          'sage-dark': '#344E61',

          primary: '#4A6B82',
          'primary-light': '#EDF3F8',
          'primary-dark': '#344E61',

          ocean: '#4A6B82',
          'ocean-light': '#EDF3F8',
          'ocean-dark': '#344E61',

          sand: '#C2B39E',
          'sand-light': '#F7F3EC',
          'sand-dark': '#9A8B77',

          rose: '#B78C83',
          'rose-light': '#F9ECE9',
          'rose-dark': '#936860',

          lavender: '#8C8392',
          'lavender-light': '#F2EEF5',
          'lavender-dark': '#69606F',
        }
      },
      fontFamily: {
        sans: ['"Noto Sans SC"', '"Plus Jakarta Sans"', 'system-ui', 'sans-serif'],
      },
      boxShadow: {
        'morandi': '0 4px 20px -2px rgba(74, 107, 130, 0.1), 0 2px 6px -1px rgba(74, 107, 130, 0.05)',
        'morandi-lg': '0 10px 30px -4px rgba(74, 107, 130, 0.16), 0 4px 12px -2px rgba(74, 107, 130, 0.08)',
      }
    },
  },
  plugins: [],
}





