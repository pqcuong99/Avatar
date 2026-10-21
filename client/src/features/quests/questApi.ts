import { get } from '../../api/http.ts';
import type { QuestsView } from '../../api/types.ts';
import { store } from '../../state/store.ts';

/** Counts claimable quests for the HUD badge. */
export async function refreshQuestBadge() {
  try {
    const e = await get<QuestsView>('/api/v1/quests/daily');
    store.set({ claimableQuests: e.result.quests.filter((q) => q.completed_at && !q.claimed_at).length });
  } catch {
    // best effort
  }
}
