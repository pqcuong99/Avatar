// Turns a waypoint list into per-step direction inputs. The last part of a
// segment uses a shorter vector so the character stops exactly on the point
// instead of overshooting (the server accepts any length up to full speed).
import { DIR_SCALE } from './movement.ts';
import type { Point } from './pathfinding.ts';

const ARRIVE = 60; // units (0.6 px)

export class PathFollower {
  private i = 0;
  private lastPos: Point | null = null;
  private stuck = 0;
  done = false;
  readonly points: Point[];
  private speed: number;

  constructor(points: Point[], speed: number) {
    this.points = points;
    this.speed = speed;
  }

  get target(): Point | null { return this.done ? null : this.points[this.points.length - 1]; }

  /** Direction for the next step from pos, or null when finished/stuck. */
  next(pos: Point): [number, number] | null {
    if (this.lastPos && this.lastPos[0] === pos[0] && this.lastPos[1] === pos[1]) {
      if (++this.stuck >= 6) { this.done = true; return null; } // blocked by something new
    } else this.stuck = 0;
    this.lastPos = [pos[0], pos[1]];
    while (this.i < this.points.length) {
      const [tx, ty] = this.points[this.i];
      const dx = tx - pos[0], dy = ty - pos[1];
      const dist = Math.hypot(dx, dy);
      if (dist <= ARRIVE) { this.i++; continue; }
      const scale = dist > this.speed ? DIR_SCALE / dist : DIR_SCALE / this.speed;
      return [Math.round(dx * scale), Math.round(dy * scale)];
    }
    this.done = true;
    return null;
  }
}
