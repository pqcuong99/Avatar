import { command, get, type Envelope } from '../api/http.ts';
import type { Catalog, GameState } from '../api/types.ts';
import { applyGameState, store, toast } from '../state/store.ts';
import { explain } from '../ui/format.ts';
import { refreshQuestBadge } from './quests/questApi.ts';

export async function refreshState() {
  const env = await get<GameState>('/api/v1/state');
  applyGameState(env.result);
}

export async function loadCatalog() {
  const env = await get<Catalog>('/api/v1/catalog');
  store.set({ catalog: env.result });
}

/**
 * Runs an asset command. The UI shows success only after the server has
 * committed (docs/04 §5); the fresh state that comes back replaces ours.
 * Returns the result, or null when the command was refused.
 */
export async function act<T>(path: string, body: Record<string, unknown> = {}, opts: { success?: (r: T) => string | void; quiet?: boolean } = {}): Promise<T | null> {
  try {
    const env: Envelope<T> = await command<T>(path, body);
    applyGameState(env.state);
    void refreshQuestBadge();
    const msg = opts.success?.(env.result);
    if (msg) toast(msg, 'success');
    return env.result;
  } catch (e) {
    if (!opts.quiet) toast(explain(e), 'error', 5000);
    if (e instanceof Error && 'code' in e && (e as { code: string }).code === 'CONFIG_VERSION_OUTDATED') {
      void loadCatalog();
    }
    void refreshState().catch(() => {});
    return null;
  }
}
