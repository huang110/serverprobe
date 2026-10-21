/** TailwindCSS 配置 */
module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        // 深色主题色板
        ink: {
          950: '#0a0d12',
          900: '#0d1117',
          800: '#12161e',
          700: '#181d27',
          600: '#1f2530',
          500: '#2a303c'
        },
        line: '#2b313c',
        muted: '#8b94a3',
        success: '#22c55e',
        warning: '#f5a524',
        danger: '#f43f5e',
        info: '#38bdf8',
        accent: '#3b82f6'
      },
      fontFamily: {
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace']
      },
      keyframes: {
        // 数据更新时的闪烁高亮
        flash: {
          '0%': { backgroundColor: 'rgba(59, 130, 246, 0.35)' },
          '100%': { backgroundColor: 'transparent' }
        },
        'fade-in': {
          '0%': { opacity: '0', transform: 'translateY(6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        'slide-in': {
          '0%': { opacity: '0', transform: 'translateX(24px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        }
      },
      animation: {
        flash: 'flash 0.8s ease-out',
        'fade-in': 'fade-in 0.25s ease-out',
        'slide-in': 'slide-in 0.25s ease-out'
      }
    }
  },
  plugins: []
}
