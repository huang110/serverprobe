/** TailwindCSS 配置 —— 颜色基于 CSS 变量，支持深色/浅色/高对比度主题 */
const withVar = (name) => `rgb(var(--${name}) / <alpha-value>)`

module.exports = {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        // 语义化色板（随主题切换）
        bg: withVar('bg'),
        surface: {
          DEFAULT: withVar('surface'),
          2: withVar('surface2'),
          3: withVar('surface3')
        },
        line: withVar('border'),
        content: withVar('text'),
        muted: withVar('muted'),
        accent: withVar('accent'),
        success: withVar('success'),
        warning: withVar('warning'),
        danger: withVar('danger'),
        info: withVar('info'),
        // 兼容旧类名：ink-* 映射到语义变量
        ink: {
          950: withVar('bg'),
          900: withVar('bg'),
          800: withVar('surface'),
          700: withVar('surface2'),
          600: withVar('surface3'),
          500: withVar('surface3')
        },
        // 覆盖 slate 的浅灰阶，使旧的 text-slate-100/200/300 跟随主题
        slate: {
          100: withVar('text'),
          200: withVar('text'),
          300: 'rgb(var(--text) / 0.72)'
        }
      },
      // 统一圆角规格：8 / 12 / 16
      borderRadius: {
        DEFAULT: '8px',
        md: '8px',
        lg: '8px',
        xl: '12px',
        '2xl': '16px'
      },
      fontFamily: {
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Consolas', 'monospace']
      },
      boxShadow: {
        glow: '0 0 18px -2px rgb(var(--accent) / 0.45)',
        'glow-success': '0 0 18px -2px rgb(var(--success) / 0.45)',
        'glow-danger': '0 0 18px -2px rgb(var(--danger) / 0.5)'
      },
      keyframes: {
        flash: {
          '0%': { backgroundColor: 'rgb(var(--accent) / 0.35)' },
          '100%': { backgroundColor: 'transparent' }
        },
        'fade-in': {
          '0%': { opacity: '0', transform: 'translateY(6px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        'slide-in': {
          '0%': { opacity: '0', transform: 'translateX(24px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        // 骨架屏微光
        shimmer: {
          '100%': { transform: 'translateX(100%)' }
        },
        // 涟漪
        ripple: {
          '0%': { transform: 'scale(0)', opacity: '0.45' },
          '100%': { transform: 'scale(4)', opacity: '0' }
        },
        // 状态告警呼吸边框
        'pulse-border': {
          '0%, 100%': { boxShadow: '0 0 0 1px rgb(var(--danger) / 0.35)' },
          '50%': { boxShadow: '0 0 0 2px rgb(var(--danger) / 0.85), 0 0 18px -2px rgb(var(--danger) / 0.6)' }
        },
        // 网络速率箭头
        'arrow-down': {
          '0%, 100%': { transform: 'translateY(-1px)', opacity: '0.55' },
          '50%': { transform: 'translateY(2px)', opacity: '1' }
        },
        'arrow-up': {
          '0%, 100%': { transform: 'translateY(1px)', opacity: '0.55' },
          '50%': { transform: 'translateY(-2px)', opacity: '1' }
        },
        // Toast 进度条
        'toast-progress': {
          '0%': { width: '100%' },
          '100%': { width: '0%' }
        },
        // 滑入指示条
        'grow-x': {
          '0%': { transform: 'scaleX(0)' },
          '100%': { transform: 'scaleX(1)' }
        }
      },
      animation: {
        flash: 'flash 0.8s ease-out',
        'fade-in': 'fade-in 0.25s ease-out',
        'slide-in': 'slide-in 0.25s ease-out',
        shimmer: 'shimmer 1.6s infinite',
        ripple: 'ripple 0.6s linear',
        'pulse-border': 'pulse-border 1.6s ease-in-out infinite',
        'arrow-down': 'arrow-down 0.9s ease-in-out infinite',
        'arrow-up': 'arrow-up 0.9s ease-in-out infinite',
        'toast-progress': 'toast-progress linear forwards'
      }
    }
  },
  plugins: []
}
