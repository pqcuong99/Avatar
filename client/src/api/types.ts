// JSON shapes returned by the Game Server / Transaction Service.

export interface Me {
  account: { id: string; username: string };
  character: { id: string; display_name: string } | null;
}

export interface ItemDef {
  item_code: string;
  display_name: string;
  category: 'seed' | 'crop' | 'worm' | 'fashion' | 'upgrade' | 'other';
  stack_limit: number;
  trade_policy: string;
}

export interface CropConfig {
  growth_seconds: number;
  base_yield: number;
  pest_probability: number;
}

export interface Crop {
  crop_code: string;
  display_name: string;
  seed_item_code: string;
  crop_item_code: string;
  sort_order: number;
  configured: boolean;
  config?: CropConfig;
}

export interface Upgrade {
  currency: Currency;
  price: number;
  slots_per_upgrade?: number;
  max_slot_cap?: number;
  max_plots?: number;
}

export interface Catalog {
  config_version: number;
  config_label: string;
  items: ItemDef[] | null;
  crops: Crop[] | null;
  inventory_upgrade: Upgrade | null;
  plot_unlock: Upgrade | null;
  xu_cap: number;
  diamond_cap: number | null;
  fashion: FashionDef[] | null;
}

export interface FashionDef {
  item_code: string;
  equip_slot: 'hat' | 'top' | 'bottom';
  style: Record<string, string>;
}

export interface Quest {
  quest_code: string;
  title: string;
  event_type: string;
  target_quantity: number;
  reward_currency: Currency;
  reward_amount: number;
  progress: number;
  completed_at: string | null;
  claimed_at: string | null;
}

export interface QuestsView {
  enabled: boolean;
  day?: string;
  resets_at?: string;
  quests: Quest[];
}

export type Currency = 'XU' | 'DIAMOND';

export interface Slot {
  slot_no: number;
  item_code: string;
  quantity: number;
  instance_id?: string;
}

export interface Pest {
  spawn_at: string;
  deadline_at: string;
  caught_at: string | null;
  active: boolean;
  missed: boolean;
  penalty_percent?: number;
}

export interface Planting {
  crop_code: string;
  planted_at: string;
  matures_at: string;
  base_yield: number;
  mature: boolean;
  pest: Pest | null;
  config_version: number;
}

export interface Plot {
  plot_no: number;
  planting: Planting | null;
}

export interface PendingCrop {
  crop_code: string;
  item_code: string;
  quantity: number;
  earliest_created_at: string;
}

export interface PendingDiamond {
  id: string;
  source_kind: string;
  source_ref: string;
  quantity: number;
  remaining: number;
  created_at: string;
}

export interface GameState {
  server_time: string;
  config_version: number;
  character: { id: string; display_name: string; inventory_slot_cap: number };
  wallets: Record<Currency, { balance: number; cap: number | null }>;
  inventory: { slot_cap: number; slots: Slot[] | null };
  farm: { plots: Plot[] | null };
  pending_crops: PendingCrop[] | null;
  pending_diamonds: PendingDiamond[] | null;
  equipment: Partial<Record<'hat' | 'top' | 'bottom', string>> | null;
}

export interface Offer {
  offer_id: string;
  item_code: string;
  item_name: string;
  category: string;
  direction: 'NPC_SELLS' | 'NPC_BUYS';
  currency: Currency;
  unit_price: number;
  held_quantity: number;
}

export interface OffersView {
  npc_code: string;
  display_name: string;
  config_version: number;
  offers: Offer[] | null;
}

export interface Player {
  id: string;
  display_name: string;
}

export interface Presence {
  online: boolean;
  room_id?: string;
}

export interface Friend extends Player {
  since: string;
  presence: Presence;
}

export interface FriendRequest {
  id: string;
  player: Player;
  created_at: string;
}

export interface Profile extends Player {
  is_friend: boolean;
  request: '' | 'incoming' | 'outgoing';
  request_id?: string;
  blocked_by_me: boolean;
}

export interface PrivateMessage {
  id: string;
  sender_id: string;
  recipient_id: string;
  body: string;
  sent_at: string;
  expires_at: string;
  seen_at: string | null;
}

export interface Conversation {
  partner: Player;
  last: PrivateMessage;
  unread: number;
}

export interface HarvestResult {
  plot_no: number;
  crop_code: string;
  item_code: string;
  quantity: number;
  destination: 'BAG' | 'PENDING';
  penalty_percent?: number;
}
