import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        background: "#010409", // Ultra-dark for contrast
        surface: "#0d1117",    // Slightly lighter for panels
        primary: "#58a6ff",    // GitHub/Bloomberg Blue
        success: "#238636",    // Deep Terminal Green
        error: "#da3633",      // Deep Terminal Red
        warning: "#d29922",    // Amber
        border: "#30363d",     // Subtle borders
        text: "#c9d1d9",       // High readability gray
        "text-muted": "#8b949e",
        "panel-bg": "#0d1117",
      },
      fontFamily: {
        mono: ["'JetBrains Mono'", "ui-monospace", "SFMono-Regular", "Menlo", "Monaco", "Consolas", "monospace"],
        sans: ["'Inter'", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "Helvetica", "Arial", "sans-serif"],
      },
      animation: {
        "fade-in": "fade-in 0.2s ease-out",
      },
      keyframes: {
        "fade-in": {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
      },
      backgroundImage: {
        "subtle-grid": "linear-gradient(to right, rgba(255, 255, 255, 0.02) 1px, transparent 1px), linear-gradient(to bottom, rgba(255, 255, 255, 0.02) 1px, transparent 1px)",
      },
    },
  },
  plugins: [],
};
export default config;
