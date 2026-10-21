import { useEffect, useRef } from 'react';
import Phaser from 'phaser';

const WORLD_WIDTH = 800;
const WORLD_HEIGHT = 480;
const PLAYER_RADIUS = 15;
// Vùng tâm nhân vật được phép đứng. Dùng chung cho di chuyển và điểm đến của click,
// để điểm đến luôn tới được và auto-path luôn kết thúc.
const WALKABLE = new Phaser.Geom.Rectangle(
  PLAYER_RADIUS,
  PLAYER_RADIUS,
  WORLD_WIDTH - 2 * PLAYER_RADIUS,
  WORLD_HEIGHT - 2 * PLAYER_RADIUS,
);

function clampToWalkable(x: number, y: number) {
  return new Phaser.Math.Vector2(
    Phaser.Math.Clamp(x, WALKABLE.left, WALKABLE.right),
    Phaser.Math.Clamp(y, WALKABLE.top, WALKABLE.bottom),
  );
}

// Phaser đo chiều cao chữ bằng testString; chuỗi mặc định "|MÉqgy" không có dấu chồng
// (Ả, Ồ, Ế...) nên dấu bị cắt. Ẩ Ẫ Ẳ Ẵ là những chữ cao nhất ở Segoe UI/Tahoma; padding
// top chừa chỗ cho phần khử răng cưa. Mọi Text (tên nhân vật, chat, NPC) phải tạo qua
// addViText, vì tên phân biệt dấu (P0-022): "Ảnh" và "Ánh" không được trông giống nhau.
const VI_TEXT_STYLE: Phaser.Types.GameObjects.Text.TextStyle = {
  testString: '|ÉqgỵẢỒẾỐỸẨẪẲẴ',
  padding: { top: 2 },
};

function addViText(
  scene: Phaser.Scene,
  x: number,
  y: number,
  text: string | string[],
  style: Phaser.Types.GameObjects.Text.TextStyle = {},
) {
  return scene.add.text(x, y, text, {
    ...VI_TEXT_STYLE,
    ...style,
    padding: { ...VI_TEXT_STYLE.padding, ...style.padding },
  });
}

class DemoScene extends Phaser.Scene {
  private player!: Phaser.GameObjects.Arc;
  private cursors!: Phaser.Types.Input.Keyboard.CursorKeys;
  private wasd!: Record<string, Phaser.Input.Keyboard.Key>;
  private destination: Phaser.Math.Vector2 | null = null;

  constructor() { super('demo'); }

  create() {
    const grid = this.add.graphics();
    grid.lineStyle(1, 0x26354a, 0.9);
    for (let x = 0; x <= WORLD_WIDTH; x += 40) grid.lineBetween(x, 0, x, WORLD_HEIGHT);
    for (let y = 0; y <= WORLD_HEIGHT; y += 40) grid.lineBetween(0, y, WORLD_WIDTH, y);
    addViText(this, 20, 18, 'BẢN ĐỒ DEMO · CHƯA KẾT NỐI GAME SERVER', { color: '#bad7f7', fontSize: '15px' });
    this.player = this.add.circle(WORLD_WIDTH / 2, WORLD_HEIGHT / 2, PLAYER_RADIUS, 0x68dd8a).setStrokeStyle(3, 0xffffff);
    this.cursors = this.input.keyboard!.createCursorKeys();
    this.wasd = this.input.keyboard!.addKeys('W,A,S,D') as Record<string, Phaser.Input.Keyboard.Key>;
    // Quy tắc P0-005: update() chỉ đọc isDown, nên một lần nhấp phím ngắn hơn 1 frame sẽ lọt.
    // Sự kiện down luôn được phát, kể cả khi phím đã nhả trước khi update() chạy.
    const { up, down, left, right } = this.cursors;
    for (const key of [up, down, left, right, this.wasd.W, this.wasd.A, this.wasd.S, this.wasd.D]) {
      key.on(Phaser.Input.Keyboard.Events.DOWN, () => { this.destination = null; });
    }
    this.input.on('pointerdown', (pointer: Phaser.Input.Pointer) => {
      this.destination = clampToWalkable(pointer.worldX, pointer.worldY);
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
      const next = clampToWalkable(this.player.x + vector.x, this.player.y + vector.y);
      this.player.setPosition(next.x, next.y);
    }
  }
}

export default function GameCanvas() {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!ref.current) return;
    const game = new Phaser.Game({
      type: Phaser.AUTO,
      width: WORLD_WIDTH,
      height: WORLD_HEIGHT,
      parent: ref.current,
      backgroundColor: '#111e2e',
      scale: { mode: Phaser.Scale.FIT, autoCenter: Phaser.Scale.CENTER_BOTH },
      scene: [DemoScene],
    });
    return () => { game.destroy(true); };
  }, []);
  return <div className="game-frame" ref={ref} aria-label="Bản đồ demo Project Thanh" />;
}
