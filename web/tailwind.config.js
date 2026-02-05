/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Redis brand colors
        redis: {
          red: '#DC382D',
          dark: '#1A1A2E',
          light: '#F5F5F5',
        },
        // Status colors
        success: '#52c41a',
        warning: '#faad14',
        error: '#ff4d4f',
        info: '#1890ff',
      },
    },
  },
  plugins: [],
  // Disable preflight to avoid conflicts with Ant Design
  corePlugins: {
    preflight: false,
  },
}
