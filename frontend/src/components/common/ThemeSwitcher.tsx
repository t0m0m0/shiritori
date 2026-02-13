import { useState, useEffect, useRef } from 'react';

const THEMES = [
  { id: 'default', label: 'デフォルト' },
  { id: 'wamo', label: '和モダン' },
  { id: 'neon', label: 'ネオン' },
  { id: 'typo', label: '活版' },
];

export function ThemeSwitcher() {
  const [current, setCurrent] = useState(() => localStorage.getItem('shiritori-theme') || 'default');
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const toggleRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node) &&
          toggleRef.current && !toggleRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('click', handleClick);
    return () => document.removeEventListener('click', handleClick);
  }, []);

  function applyTheme(name: string) {
    document.documentElement.setAttribute('data-theme', name);
    localStorage.setItem('shiritori-theme', name);
    const link = document.getElementById('themeCSS') as HTMLLinkElement | null;
    if (link) {
      link.href = name === 'default' ? '' : `/static/themes/${name}.css`;
    }
    setCurrent(name);
    setOpen(false);
  }

  return (
    <div className="theme-switcher">
      <button className="theme-toggle" ref={toggleRef} aria-label="テーマ切替" onClick={(e) => { e.stopPropagation(); setOpen(!open); }}>
        テーマ
      </button>
      <div className={`theme-panel${open ? ' open' : ''}`} ref={panelRef}>
        <div className="theme-panel-title">テーマ</div>
        {THEMES.map((t) => (
          <button
            key={t.id}
            className={`theme-btn${current === t.id ? ' active' : ''}`}
            onClick={() => applyTheme(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>
    </div>
  );
}
