import { get } from '../../api/http.ts';
import type { Conversation, FriendRequest } from '../../api/types.ts';
import { store } from '../../state/store.ts';

/** Refreshes the HUD counters: incoming friend requests and unread messages. */
export async function refreshSocialCounters() {
  try {
    const [req, conv] = await Promise.all([
      get<{ incoming: FriendRequest[] }>('/api/v1/friends/requests'),
      get<{ conversations: Conversation[] }>('/api/v1/private-chat/conversations'),
    ]);
    store.set({
      incomingRequests: req.result.incoming.length,
      unreadPM: conv.result.conversations.reduce((n, c) => n + c.unread, 0),
    });
  } catch {
    // counters are best effort
  }
}
