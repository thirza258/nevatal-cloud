import React, { useId, useState } from 'react';

export type R640Variant = 'blueprint' | 'front';

interface R640ChassisProps {
  variant?: R640Variant;
  className?: string;
}

/*
 * Dell PowerEdge R640, 10 x 2.5" drive configuration, front view.
 *
 * Layout follows Dell's PowerEdge R640 Installation and Service Manual,
 * "Front view of the system", Figure 4 (10 x 2.5 inch hard drive system):
 *   - left rack ear: left control panel (system health / system ID bar,
 *     five status LEDs: drive, temperature, electrical, memory, PCIe)
 *   - 10 hot-swap 2.5" carriers in two rows of five, full-width vent strip above
 *   - VGA port to the right of the drive cage
 *   - right rack ear: power button, USB 2.0 port, iDRAC Direct micro-USB + LED
 *
 * Drawing units: the panel is 2000 x 176 units for 482.0 mm (with rack ears)
 * x 42.8 mm (1U), i.e. ~4.15 units per mm.
 */

const W = 2000;
const H = 176;
const COLS = 5;
const ROWS = 2;
const BAY_X0 = 132;
const BAY_PITCH = 336;
const BAY_W = 296;
const BAY_H = 66;
const ROW_Y = [32, 104];
const VGA_X = 1826;
const PANEL_X = 1906;

// Deterministic activity-LED rhythm per bay (seconds).
const BLINK = [
  [1.7, 0.0], [2.3, 0.4], [1.1, 0.9], [2.9, 0.2], [1.5, 1.3],
  [2.1, 0.7], [0.9, 0.1], [2.6, 1.1], [1.3, 0.5], [1.9, 0.8],
];

type Palette = {
  body: string;
  bodyStroke: string;
  ear: string;
  cutout: string;
  cutoutStroke: string;
  vent: string;
  carrier: string;
  carrierStroke: string;
  plate: string;
  ring: string;
  grille: string;
  hex: string;
  handle: string;
  port: string;
  portStroke: string;
  tongue: string;
  vgaShell: string;
  pin: string;
  ledActivity: string;
  ledStatus: string;
  ledOff: string;
  ledOffStroke: string;
  health: string;
  power: string;
  highlight: string;
  ink: string;
  inkMuted: string;
  accent: string;
};

const PALETTES: Record<R640Variant, Palette> = {
  front: {
    body: 'url(#BODY)',
    bodyStroke: '#3f3f46',
    ear: '#1c1d21',
    cutout: '#08080a',
    cutoutStroke: '#2a2c31',
    vent: '#0a0a0c',
    carrier: 'url(#CARRIER)',
    carrierStroke: '#34373e',
    plate: '#141518',
    ring: '#f0883e',
    grille: '#0b0c0f',
    hex: '#2f333a',
    handle: 'url(#HANDLE)',
    port: '#0b0b0d',
    portStroke: '#3f3f46',
    tongue: '#a1a1aa',
    vgaShell: '#4b5563',
    pin: '#9ca3af',
    ledActivity: '#22c55e',
    ledStatus: '#16a34a',
    ledOff: '#26272b',
    ledOffStroke: '#3a3b40',
    health: '#3b82f6',
    power: '#22c55e',
    highlight: '#34d399',
    ink: '#e4e4e7',
    inkMuted: '#71717a',
    accent: '#34d399',
  },
  blueprint: {
    body: 'rgba(24,24,27,0.35)',
    bodyStroke: '#d4d4d8',
    ear: 'none',
    cutout: 'none',
    cutoutStroke: '#a1a1aa',
    vent: 'none',
    carrier: 'none',
    carrierStroke: '#a1a1aa',
    plate: 'none',
    ring: '#d4d4d8',
    grille: 'none',
    hex: '#52525b',
    handle: 'none',
    port: 'none',
    portStroke: '#a1a1aa',
    tongue: 'none',
    vgaShell: 'none',
    pin: '#a1a1aa',
    ledActivity: '#34d399',
    ledStatus: '#34d399',
    ledOff: 'none',
    ledOffStroke: '#71717a',
    health: '#60a5fa',
    power: '#34d399',
    highlight: '#34d399',
    ink: '#e4e4e7',
    inkMuted: '#71717a',
    accent: '#34d399',
  },
};

const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';

export const R640Chassis: React.FC<R640ChassisProps> = ({ variant = 'blueprint', className }) => {
  const uid = useId().replace(/[^a-zA-Z0-9_-]/g, '');
  const ref = (name: string) => `${name}-${uid}`;
  const paint = (value: string) => value.replace(/url\(#([A-Z]+)\)/, (_, n) => `url(#${ref(n)})`);
  const p = Object.fromEntries(
    Object.entries(PALETTES[variant]).map(([k, v]) => [k, paint(v)])
  ) as Palette;
  const blueprint = variant === 'blueprint';
  const line = blueprint ? 2 : 1.5;

  const [hover, setHover] = useState<{ row: number; col: number } | null>(null);

  const viewBox = blueprint ? '-90 -150 2140 470' : '-12 -12 2024 222';

  return (
    <figure className={className}>
      <div className="w-full overflow-x-auto [scrollbar-width:thin]">
        <svg
          viewBox={viewBox}
          className="w-full min-w-[640px] h-auto select-none"
          role="img"
          aria-labelledby={`${ref('t')} ${ref('d')}`}
        >
          <title id={ref('t')}>Dell PowerEdge R640 front panel, 10 × 2.5-inch drive configuration</title>
          <desc id={ref('d')}>
            1U rack server. Left control panel with system health and ID bar and five status LEDs; ten
            hot-swap 2.5-inch drive carriers in two rows of five; VGA port; right control panel with power
            button, USB 2.0 port and iDRAC Direct micro-USB port.
          </desc>

          <style>{`
            .r640-act-${uid} { animation: r640-blink-${uid} var(--d) steps(2, jump-none) var(--o) infinite; }
            @keyframes r640-blink-${uid} { 0%, 60% { opacity: 1 } 61%, 100% { opacity: .18 } }
            .r640-bay-${uid} { cursor: default; outline: none; }
            @media (prefers-reduced-motion: reduce) { .r640-act-${uid} { animation: none; } }
          `}</style>

          <defs>
            <linearGradient id={ref('BODY')} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="#2c2f35" />
              <stop offset="0.5" stopColor="#1f2125" />
              <stop offset="1" stopColor="#16171a" />
            </linearGradient>
            <linearGradient id={ref('CARRIER')} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="#25272c" />
              <stop offset="1" stopColor="#191a1e" />
            </linearGradient>
            <linearGradient id={ref('HANDLE')} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="#5a5e66" />
              <stop offset="1" stopColor="#373a40" />
            </linearGradient>
            <pattern id={ref('hex')} width="8.66" height="15" patternUnits="userSpaceOnUse">
              {[
                [0, 0], [8.66, 0], [4.33, 7.5], [0, 15], [8.66, 15],
              ].map(([x, y], i) => (
                <path
                  key={i}
                  d={`M${x} ${y - 4.6}l3.98 2.3v4.6l-3.98 2.3l-3.98-2.3v-4.6z`}
                  fill="none"
                  stroke={p.hex}
                  strokeWidth={blueprint ? 0.8 : 1.1}
                />
              ))}
            </pattern>
            <filter id={ref('glow')} x="-200%" y="-200%" width="500%" height="500%">
              <feGaussianBlur stdDeviation="3.5" result="b" />
              <feMerge>
                <feMergeNode in="b" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
            <filter id={ref('floor')} x="-10%" y="-200%" width="120%" height="500%">
              <feGaussianBlur stdDeviation="9" />
            </filter>
            <marker id={ref('arrow')} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
              <path d="M0 0L10 5L0 10z" fill={p.inkMuted} />
            </marker>
          </defs>

          {!blueprint && (
            <ellipse cx={W / 2} cy={H + 14} rx={W * 0.46} ry={9} fill="#000" opacity={0.65} filter={`url(#${ref('floor')})`} />
          )}

          {/* Chassis */}
          <rect x={0} y={0} width={W} height={H} rx={8} fill={p.body} stroke={p.bodyStroke} strokeWidth={line} />
          {!blueprint && <line x1={10} y1={1.5} x2={W - 10} y2={1.5} stroke="#fff" strokeOpacity={0.13} strokeWidth={1.5} />}

          {/* Rack ears */}
          {[0, W - 64].map((x, i) => (
            <g key={`ear-${i}`}>
              <rect x={x} y={0} width={i === 0 ? 96 : 64} height={H} rx={8} fill={p.ear} stroke={p.bodyStroke} strokeOpacity={0.6} strokeWidth={line} />
              <rect x={x + (i === 0 ? 10 : 12)} y={26} width={i === 0 ? 42 : 40} height={124} rx={12} fill={p.cutout} stroke={p.cutoutStroke} strokeWidth={line} />
            </g>
          ))}

          {/* Left control panel: system health / system ID bar */}
          <circle cx={76} cy={30} r={5} fill="none" stroke={p.inkMuted} strokeWidth={1.5} />
          <rect
            x={70}
            y={44}
            width={12}
            height={88}
            rx={6}
            fill={blueprint ? 'none' : p.health}
            stroke={p.health}
            strokeWidth={blueprint ? 2 : 1}
            filter={blueprint ? undefined : `url(#${ref('glow')})`}
          />

          {/* Status LEDs (off unless a fault): drive, temperature, electrical, memory, PCIe */}
          {[30, 58, 86, 114, 142].map((y) => (
            <rect key={y} x={104} y={y} width={14} height={12} rx={2.5} fill={p.ledOff} stroke={p.ledOffStroke} strokeWidth={1.2} />
          ))}

          {/* Vent strip above the drive cage */}
          {Array.from({ length: COLS }, (_, c) =>
            Array.from({ length: 19 }, (_, i) => (
              <rect
                key={`v-${c}-${i}`}
                x={BAY_X0 + c * BAY_PITCH + 14 + i * 15}
                y={10}
                width={9}
                height={13}
                rx={1.5}
                fill={p.vent}
                stroke={blueprint ? p.hex : 'none'}
                strokeWidth={0.9}
              />
            ))
          )}

          {/* Separators between carrier columns */}
          {Array.from({ length: COLS - 1 }, (_, c) => {
            const x = BAY_X0 + c * BAY_PITCH + BAY_W + 12;
            return (
              <g key={`sep-${c}`}>
                {Array.from({ length: 8 }, (_, k) => (
                  <rect key={k} x={x + (k % 2) * 9} y={40 + Math.floor(k / 2) * 34} width={6} height={12} rx={1.5} fill={p.vent} stroke={blueprint ? p.hex : 'none'} strokeWidth={0.9} />
                ))}
              </g>
            );
          })}

          {/* Drive carriers: 2 rows x 5 columns */}
          {Array.from({ length: ROWS }, (_, row) =>
            Array.from({ length: COLS }, (_, col) => {
              const x = BAY_X0 + col * BAY_PITCH;
              const y = ROW_Y[row];
              const [dur, off] = BLINK[col * ROWS + row];
              const hovered = hover?.row === row && hover?.col === col;
              return (
                <g
                  key={`bay-${row}-${col}`}
                  className={`r640-bay-${uid}`}
                  tabIndex={0}
                  aria-label={`Front drive bay, row ${row + 1}, slot ${col + 1}`}
                  onMouseEnter={() => setHover({ row, col })}
                  onMouseLeave={() => setHover(null)}
                  onFocus={() => setHover({ row, col })}
                  onBlur={() => setHover(null)}
                >
                  <title>{`Row ${row + 1} · slot ${col + 1} — 2.5″ hot-swap carrier`}</title>
                  <rect x={x} y={y} width={BAY_W} height={BAY_H} rx={4} fill={p.carrier} stroke={p.carrierStroke} strokeWidth={line} />
                  {/* activity + status LEDs */}
                  <rect
                    x={x + 9}
                    y={y + 12}
                    width={8}
                    height={14}
                    rx={2}
                    fill={p.ledActivity}
                    className={`r640-act-${uid}`}
                    style={{ ['--d' as string]: `${dur}s`, ['--o' as string]: `${off}s` } as React.CSSProperties}
                  />
                  <rect x={x + 9} y={y + 40} width={8} height={14} rx={2} fill={blueprint ? 'none' : p.ledStatus} stroke={p.ledStatus} strokeWidth={1.2} />
                  {/* release button */}
                  <rect x={x + 26} y={y + 7} width={50} height={52} rx={4} fill={p.plate} stroke={blueprint ? p.carrierStroke : 'none'} strokeWidth={1.2} />
                  <circle cx={x + 51} cy={y + 33} r={12} fill={blueprint ? 'none' : '#111214'} stroke={p.ring} strokeWidth={blueprint ? 2 : 4.5} />
                  {/* hex grille windows */}
                  {[0, 1, 2].map((k) => (
                    <rect
                      key={k}
                      x={x + 90 + k * 56}
                      y={y + 13}
                      width={50}
                      height={40}
                      rx={3}
                      fill={blueprint ? `url(#${ref('hex')})` : p.grille}
                      stroke={blueprint ? p.carrierStroke : '#2a2d33'}
                      strokeWidth={1}
                    />
                  ))}
                  {!blueprint &&
                    [0, 1, 2].map((k) => (
                      <rect key={`m-${k}`} x={x + 90 + k * 56} y={y + 13} width={50} height={40} rx={3} fill={`url(#${ref('hex')})`} />
                    ))}
                  {/* handle */}
                  <rect x={x + 258} y={y + 7} width={32} height={52} rx={3} fill={p.handle} stroke={blueprint ? p.carrierStroke : 'none'} strokeWidth={1.2} />
                  <rect x={x + 26} y={y + 58} width={264} height={4} rx={2} fill={blueprint ? 'none' : p.handle} stroke={blueprint ? p.carrierStroke : 'none'} strokeWidth={1} />
                  {hovered && (
                    <rect x={x - 3} y={y - 3} width={BAY_W + 6} height={BAY_H + 6} rx={6} fill={p.highlight} fillOpacity={0.07} stroke={p.highlight} strokeWidth={3} />
                  )}
                </g>
              );
            })
          )}

          {/* VGA port */}
          <circle cx={VGA_X} cy={40} r={5} fill={blueprint ? 'none' : '#2a2c31'} stroke={p.portStroke} strokeWidth={1.2} />
          <circle cx={VGA_X} cy={136} r={5} fill={blueprint ? 'none' : '#2a2c31'} stroke={p.portStroke} strokeWidth={1.2} />
          <path
            d={`M${VGA_X - 21} 54h42l-5 68h-32z`}
            fill={p.vgaShell}
            stroke={blueprint ? p.portStroke : '#6b7280'}
            strokeWidth={blueprint ? 2 : 3}
            strokeLinejoin="round"
          />
          <path d={`M${VGA_X - 14} 62h28l-3.5 52h-21z`} fill={blueprint ? 'none' : '#14161a'} stroke={blueprint ? p.portStroke : 'none'} strokeWidth={1.2} strokeLinejoin="round" />
          {[-7, 0, 7].map((dx) =>
            [0, 1, 2, 3, 4].map((k) => <circle key={`${dx}-${k}`} cx={VGA_X + dx} cy={70 + k * 9.5} r={1.8} fill={p.pin} />)
          )}

          {/* Right control panel: power, USB 2.0, iDRAC Direct (micro-USB + LED) */}
          <circle cx={PANEL_X} cy={30} r={13} fill={blueprint ? 'none' : '#0f1012'} stroke={p.portStroke} strokeWidth={1.5} />
          <g stroke={p.power} strokeWidth={2.6} strokeLinecap="round" fill="none" filter={blueprint ? undefined : `url(#${ref('glow')})`}>
            <path d={`M${PANEL_X - 4.6} 26.2a6.4 6.4 0 1 0 9.2 0`} />
            <line x1={PANEL_X} y1={22} x2={PANEL_X} y2={29} />
          </g>
          <rect x={PANEL_X - 12} y={56} width={24} height={54} rx={3} fill={p.port} stroke={p.portStroke} strokeWidth={1.5} />
          <rect x={PANEL_X - 8} y={62} width={8} height={42} rx={1.5} fill={blueprint ? 'none' : p.tongue} stroke={blueprint ? p.portStroke : 'none'} strokeWidth={1} />
          <path d={`M${PANEL_X - 7} 124h14v24l-3 4h-8l-3-4z`} fill={p.port} stroke={p.portStroke} strokeWidth={1.5} strokeLinejoin="round" />
          <circle cx={PANEL_X + 14} cy={118} r={3} fill={p.ledOff} stroke={p.ledOffStroke} strokeWidth={1} />

          {blueprint && (
            <g fontFamily={MONO} fill={p.ink}>
              {/* Overall width */}
              <g stroke={p.inkMuted} strokeWidth={1.4}>
                <line x1={0} y1={-12} x2={0} y2={-124} />
                <line x1={W} y1={-12} x2={W} y2={-124} />
                <line x1={0} y1={-112} x2={W / 2 - 120} y2={-112} markerStart={`url(#${ref('arrow')})`} />
                <line x1={W / 2 + 120} y1={-112} x2={W} y2={-112} markerEnd={`url(#${ref('arrow')})`} />
              </g>
              <text x={W / 2} y={-103} fontSize={26} textAnchor="middle" fill={p.inkMuted}>482.0 mm</text>

              {/* Height */}
              <g stroke={p.inkMuted} strokeWidth={1.4}>
                <line x1={-12} y1={0} x2={-62} y2={0} />
                <line x1={-12} y1={H} x2={-62} y2={H} />
                <line x1={-50} y1={0} x2={-50} y2={H} markerStart={`url(#${ref('arrow')})`} markerEnd={`url(#${ref('arrow')})`} />
              </g>
              <text x={-62} y={H / 2} fontSize={22} textAnchor="middle" fill={p.inkMuted} transform={`rotate(-90 -62 ${H / 2})`} dy={-6}>
                42.8 mm · 1U
              </text>

              {/* Callouts (Dell ISM, Table 1) */}
              {[
                { n: 1, cx: 76, cy: -52, tx: 76, ty: 40, label: 'Left control panel — health/ID, status LEDs', anchor: 'start' as const },
                { n: 2, cx: BAY_X0 + 2 * BAY_PITCH + 150, cy: -52, tx: BAY_X0 + 2 * BAY_PITCH + 150, ty: ROW_Y[0], label: '10 × 2.5″ hot-swap bays (2 × 5)', anchor: 'start' as const },
                { n: 3, cx: VGA_X, cy: 226, tx: VGA_X, ty: 143, label: 'VGA port', anchor: 'end' as const },
                { n: 4, cx: PANEL_X, cy: 284, tx: PANEL_X, ty: H, label: 'Right control panel — power, USB 2.0, iDRAC Direct', anchor: 'end' as const },
              ].map((c) => (
                <g key={c.n}>
                  <line x1={c.cx} y1={c.cy + (c.cy < 0 ? 18 : -18)} x2={c.tx} y2={c.ty} stroke={p.accent} strokeWidth={1.6} strokeDasharray="5 5" />
                  <circle cx={c.tx} cy={c.ty} r={4} fill={p.accent} />
                  <circle cx={c.cx} cy={c.cy} r={18} fill="#09090b" stroke={p.accent} strokeWidth={2} />
                  <text x={c.cx} y={c.cy + 8} fontSize={22} textAnchor="middle" fill={p.accent}>{c.n}</text>
                  <text x={c.anchor === 'start' ? c.cx + 32 : c.cx - 32} y={c.cy + 8} fontSize={24} textAnchor={c.anchor} fill={p.ink}>
                    {c.label}
                  </text>
                </g>
              ))}

              <text x={0} y={238} fontSize={22} fill={p.inkMuted} letterSpacing={2}>
                POWEREDGE R640 · FRONT VIEW · 10 × 2.5″
              </text>
            </g>
          )}
        </svg>
      </div>

      <figcaption className="mt-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 font-mono text-[11px] text-zinc-400">
        <span className="w-full text-zinc-500 sm:hidden">Swipe sideways to see the full chassis →</span>
        <span aria-live="polite">
          {hover
            ? `Row ${hover.row + 1} · slot ${hover.col + 1} — 2.5″ hot-swap carrier (NVMe U.2 / SAS / SATA)`
            : '10 × 2.5″ hot-swap bays in 2 rows × 5 · front I/O: VGA, USB 2.0, iDRAC Direct'}
        </span>
        <span className="flex items-center gap-3 text-zinc-500">
          <span className="flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />drive activity</span>
          <span className="flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-blue-400" />system health</span>
          <span className="flex items-center gap-1.5"><span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />power</span>
        </span>
      </figcaption>
    </figure>
  );
};
