import { useEffect, useRef } from 'react';
import { ART_H, ART_W, drawCharacter, paletteFor, paletteWith, type Appearance } from '../../game/art/characterArt.ts';

const SCALE = 3;

export function CharacterPreview({ seed, name, look }: { seed: string; name: string; look?: Appearance | null }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const lookJSON = JSON.stringify(look ?? null);
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const palette = paletteWith(paletteFor(seed), JSON.parse(lookJSON) as Appearance | null);
    let frame = 0;
    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.save();
      ctx.scale(SCALE, SCALE);
      ctx.fillStyle = 'rgba(0,0,0,0.18)';
      ctx.beginPath();
      ctx.ellipse(16, 46, 9, 2.6, 0, 0, Math.PI * 2);
      ctx.fill();
      drawCharacter(ctx, palette, 'front', frame < 4 ? -1 : frame - 4);
      ctx.restore();
    };
    draw();
    const t = setInterval(() => { frame = (frame + 1) % 8; draw(); }, 220);
    return () => clearInterval(t);
  }, [seed, lookJSON]);
  return (
    <div className="char-preview">
      <canvas ref={ref} width={ART_W * SCALE} height={ART_H * SCALE} aria-hidden="true" />
      <div className="char-preview-name">{name}</div>
    </div>
  );
}
