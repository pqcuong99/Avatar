import { useEffect, useRef } from 'react';
import Phaser from 'phaser';

class DemoScene extends Phaser.Scene {
  private player!: Phaser.GameObjects.Arc;
  private cursors!: Phaser.Types.Input.Keyboard.CursorKeys;
  private wasd!: Record<string, Phaser.Input.Keyboard.Key>;
  private destination: Phaser.Math.Vector2 | null = null;

  constructor() { super('demo'); }

  create() {
    const grid = this.add.graphics();
    grid.lineStyle(1, 0x26354a, 0.9);
    for (let x = 0; x <= 800; x += 40) grid.lineBetween(x, 0, x, 480);
    for (let y = 0; y <= 480; y += 40) grid.lineBetween(0, y, 800, y);
    this.add.text(20, 18, 'BẢN ĐỒ DEMO · CHƯA KẾT NỐI GAME SERVER', { color: '#bad7f7', fontSize: '15px' });
    this.player = this.add.circle(400, 240, 15, 0x68dd8a).setStrokeStyle(3, 0xffffff);
    this.cursors = this.input.keyboard!.createCursorKeys();
    this.wasd = this.input.keyboard!.addKeys('W,A,S,D') as Record<string, Phaser.Input.Keyboard.Key>;
    this.input.on('pointerdown', (pointer: Phaser.Input.Pointer) => {
      this.destination = new Phaser.Math.Vector2(pointer.worldX, pointer.worldY);
    });
  }

  update(_: number, delta: number) {
    const horizontal = Number(this.cursors.right.isDown || this.wasd.D.isDown)
      - Number(this.cursors.left.isDown || this.wasd.A.isDown);
    const vertical = Number(this.cursors.down.isDown || this.wasd.S.isDown)
      - Number(this.cursors.up.isDown || this.wasd.W.isDown);
    const vector = new Phaser.Math.Vector2(horizontal, vertical);
    // Quy tắc P0-005: điều khiển thủ công hủy tự di chuyển.
    if (vector.lengthSq() > 0) this.destination = null;
    else if (this.destination) {
      vector.set(this.destination.x - this.player.x, this.destination.y - this.player.y);
      if (vector.length() < 4) { vector.set(0, 0); this.destination = null; }
    }
    if (vector.lengthSq() > 0) {
      vector.normalize().scale(160 * Math.min(delta, 100) / 1000);
      this.player.setPosition(
        Phaser.Math.Clamp(this.player.x + vector.x, 15, 785),
        Phaser.Math.Clamp(this.player.y + vector.y, 15, 465),
      );
    }
  }
}

export default function GameCanvas() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!ref.current) return;
    const game = new Phaser.Game({
      type: Phaser.AUTO,
      width: 800,
      height: 480,
      parent: ref.current,
      backgroundColor: '#111e2e',
      scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH },
      scene: [DemoScene],
    });
    return () => { game.destroy(true); };
  }, []);
  return <div className="game-frame" ref={ref} aria-label="Bản đồ demo Project Thanh" />;
}
