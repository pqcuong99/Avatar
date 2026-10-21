// Original vector icons for the MVP items (placeholders until the art set is
// decided, GAP-13). Seeds are a small sack marked with their crop.
import { store } from '../state/store.ts';

const CROP_COLORS: Record<string, string> = {
  carrot: '#f08c2b',
  potato: '#c49a5a',
  tomato: '#e5383b',
  corn: '#f6c445',
  cabbage: '#6cc04a',
};

function Crop({ code }: { code: string }) {
  switch (code) {
    case 'carrot':
      return (
        <g>
          <path d="M20 8 l-3 -6 M20 8 l0 -7 M20 8 l4 -6" stroke="#3f9a35" strokeWidth="2.5" strokeLinecap="round" fill="none" />
          <path d="M14 10 Q20 6 26 10 L19 37 Z" fill="#f08c2b" stroke="#c2621a" strokeWidth="1.2" />
          <path d="M17 16 h4 M18 22 h3 M19 28 h2" stroke="#c2621a" strokeWidth="1.2" />
        </g>
      );
    case 'potato':
      return (
        <g>
          <ellipse cx="20" cy="22" rx="14" ry="11" fill="#c49a5a" stroke="#8a6631" strokeWidth="1.4" />
          <circle cx="14" cy="19" r="1.4" fill="#8a6631" />
          <circle cx="23" cy="16" r="1.2" fill="#8a6631" />
          <circle cx="25" cy="26" r="1.4" fill="#8a6631" />
          <circle cx="16" cy="27" r="1" fill="#8a6631" />
        </g>
      );
    case 'tomato':
      return (
        <g>
          <circle cx="20" cy="23" r="13" fill="#e5383b" stroke="#a4161a" strokeWidth="1.4" />
          <path d="M20 11 l3 4 l4 -1 l-3 4 l-4 -2 l-4 2 l-3 -4 l4 1 z" fill="#3f9a35" />
          <ellipse cx="15" cy="19" rx="3" ry="2" fill="#ff8a8c" opacity="0.7" />
        </g>
      );
    case 'corn':
      return (
        <g>
          <path d="M11 34 Q8 18 16 8 Q14 24 18 36 Z" fill="#5aa845" />
          <path d="M29 34 Q32 18 24 8 Q26 24 22 36 Z" fill="#4c9a3c" />
          <ellipse cx="20" cy="20" rx="6" ry="14" fill="#f6c445" stroke="#c99a1b" strokeWidth="1.2" />
          <path d="M16 14 h8 M15 19 h10 M15 24 h10 M16 29 h8" stroke="#e0a91d" strokeWidth="1.2" />
        </g>
      );
    case 'cabbage':
      return (
        <g>
          <circle cx="20" cy="22" r="14" fill="#6cc04a" stroke="#3f8a2c" strokeWidth="1.4" />
          <circle cx="20" cy="22" r="9" fill="#93d66f" />
          <path d="M20 10 Q24 22 20 34 M10 18 Q20 22 30 18" stroke="#3f8a2c" strokeWidth="1.2" fill="none" />
        </g>
      );
    default:
      return <circle cx="20" cy="20" r="12" fill="#aaa" />;
  }
}

function Worm() {
  return (
    <g>
      {[8, 14, 20, 26].map((x, i) => (
        <circle key={x} cx={x} cy={24 - (i % 2) * 4} r="5" fill="#f4a3b8" stroke="#c9667f" strokeWidth="1.2" />
      ))}
      <circle cx="31" cy="18" r="5.5" fill="#f4a3b8" stroke="#c9667f" strokeWidth="1.2" />
      <circle cx="33" cy="17" r="1.2" fill="#2b2d42" />
    </g>
  );
}

function Seed({ crop }: { crop: string }) {
  const c = CROP_COLORS[crop] ?? '#999';
  return (
    <g>
      <path d="M9 14 Q8 36 20 37 Q32 36 31 14 Z" fill="#e9d8a6" stroke="#a68a4d" strokeWidth="1.4" />
      <path d="M11 14 Q20 8 29 14 Q20 18 11 14 Z" fill="#d4bd85" stroke="#a68a4d" strokeWidth="1.2" />
      <path d="M13 12 l14 0" stroke="#7f5539" strokeWidth="2" />
      <circle cx="20" cy="26" r="6" fill={c} stroke="#00000033" />
      <circle cx="17" cy="35" r="1" fill="#7f5539" />
      <circle cx="23" cy="35" r="1" fill="#7f5539" />
    </g>
  );
}

function Garment({ slot, style }: { slot: string; style: Record<string, string> }) {
  if (slot === 'top') {
    const c = style.shirt ?? '#4dabf7', d = style.shirt_dark ?? '#1c7ed6';
    return (
      <g>
        <path d="M12 8 L6 13 L9 19 L12 17 L12 34 L28 34 L28 17 L31 19 L34 13 L28 8 Q20 13 12 8 Z" fill={c} stroke={d} strokeWidth="1.5" strokeLinejoin="round" />
        <path d="M16 9 Q20 14 24 9" stroke={d} strokeWidth="1.4" fill="none" />
      </g>
    );
  }
  if (slot === 'bottom') {
    const c = style.pants ?? '#364fc7';
    return <path d="M11 8 H29 L31 34 H22 L20 18 L18 34 H9 Z" fill={c} stroke="#00000055" strokeWidth="1.4" strokeLinejoin="round" />;
  }
  if (style.hat === 'straw') {
    return (
      <g>
        <ellipse cx="20" cy="26" rx="16" ry="6" fill="#f2d16b" stroke="#b8902d" strokeWidth="1.3" />
        <path d="M11 25 Q11 12 20 12 Q29 12 29 25 Z" fill="#f2d16b" stroke="#b8902d" strokeWidth="1.3" />
        <rect x="11" y="21" width="18" height="3" fill="#c0392b" />
      </g>
    );
  }
  if (style.hat === 'flower') {
    return (
      <g>
        {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => {
          const a = (i / 8) * Math.PI * 2;
          return <circle key={i} cx={20 + Math.cos(a) * 11} cy={21 + Math.sin(a) * 9} r="4" fill={i % 2 ? '#ffd43b' : style.color ?? '#f783ac'} stroke="#00000033" />;
        })}
      </g>
    );
  }
  const c = style.color ?? '#1c7ed6';
  return (
    <g>
      <path d="M8 24 Q8 10 20 10 Q32 10 32 24 Z" fill={c} stroke="#00000055" strokeWidth="1.3" />
      <ellipse cx="22" cy="25" rx="14" ry="3.5" fill={c} stroke="#00000055" strokeWidth="1.2" />
      <circle cx="20" cy="10" r="2" fill="#fff" />
    </g>
  );
}

function RenameCard() {
  return (
    <g>
      <rect x="5" y="9" width="30" height="22" rx="3" fill="#fff3bf" stroke="#c9a227" strokeWidth="1.4" />
      <path d="M10 17 h12 M10 23 h8" stroke="#8d6e1a" strokeWidth="1.6" strokeLinecap="round" />
      <path d="M24 27 l8 -9 l3 3 l-8 9 l-4 1 z" fill="#4dabf7" stroke="#1864ab" strokeWidth="1.1" />
    </g>
  );
}

export function ItemIcon({ code, size = 40 }: { code: string; size?: number }) {
  const fashion = store.get().catalog?.fashion?.find((f) => f.item_code === code);
  let body;
  if (fashion) body = <Garment slot={fashion.equip_slot} style={fashion.style} />;
  else if (code === 'worm') body = <Worm />;
  else if (code.startsWith('seed_')) body = <Seed crop={code.slice(5)} />;
  else if (code.startsWith('crop_')) body = <Crop code={code.slice(5)} />;
  else if (code === 'rename_card') body = <RenameCard />;
  else body = <rect x="8" y="8" width="24" height="24" rx="5" fill="#b8c0cc" />;
  return (
    <svg width={size} height={size} viewBox="0 0 40 40" aria-hidden="true">
      {body}
    </svg>
  );
}

export function CurrencyIcon({ currency, size = 18 }: { currency: 'XU' | 'DIAMOND'; size?: number }) {
  if (currency === 'XU') {
    return (
      <svg width={size} height={size} viewBox="0 0 20 20" aria-hidden="true">
        <circle cx="10" cy="10" r="8.5" fill="#f6c445" stroke="#b7791f" strokeWidth="1.5" />
        <circle cx="10" cy="10" r="5" fill="none" stroke="#b7791f" strokeWidth="1.2" />
      </svg>
    );
  }
  return (
    <svg width={size} height={size} viewBox="0 0 20 20" aria-hidden="true">
      <path d="M4 7 L7 3 H13 L16 7 L10 17 Z" fill="#74c0fc" stroke="#1971c2" strokeWidth="1.3" />
      <path d="M4 7 H16 M7 3 L10 7 L13 3 M10 7 L10 17" stroke="#1971c2" strokeWidth="0.8" fill="none" />
    </svg>
  );
}
