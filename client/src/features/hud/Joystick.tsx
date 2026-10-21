import { useRef, useState, type PointerEvent } from 'react';

const R = 48;

// P0-003: virtual joystick on the left for touch screens (landscape).
export function Joystick({ onChange }: { onChange: (v: [number, number] | null) => void }) {
  const base = useRef<HTMLDivElement>(null);
  const [knob, setKnob] = useState<[number, number]>([0, 0]);
  const active = useRef<number | null>(null);

  function update(e: PointerEvent) {
    const rect = base.current!.getBoundingClientRect();
    let dx = e.clientX - (rect.left + rect.width / 2);
    let dy = e.clientY - (rect.top + rect.height / 2);
    const len = Math.hypot(dx, dy);
    if (len > R) { dx = (dx / len) * R; dy = (dy / len) * R; }
    setKnob([dx, dy]);
    onChange(len < R * 0.2 ? null : [dx / R, dy / R]);
  }

  function end() {
    active.current = null;
    setKnob([0, 0]);
    onChange(null);
  }

  return (
    <div
      ref={base}
      className="joystick"
      onPointerDown={(e) => { active.current = e.pointerId; e.currentTarget.setPointerCapture(e.pointerId); update(e); }}
      onPointerMove={(e) => { if (active.current === e.pointerId) update(e); }}
      onPointerUp={end}
      onPointerCancel={end}
      aria-label="Cần điều khiển"
    >
      <div className="joystick-knob" style={{ transform: `translate(${knob[0]}px, ${knob[1]}px)` }} />
    </div>
  );
}
