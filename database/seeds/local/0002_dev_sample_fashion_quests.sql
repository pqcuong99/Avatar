-- ===========================================================================
-- DEV SAMPLE CONTENT — LOCAL DEVELOPMENT ONLY. NOT PRODUCTION VALUES.
--
-- Fashion catalog, fashion prices, daily quest list, targets, rewards and the
-- quest reset time zone are all [CẦN CẤU HÌNH] (GAP-10, GAP-13, GAP-14).
-- These placeholders exist so the fashion and quest engines can be played
-- locally. Published config versions are immutable, so this file does what
-- maintenance would do: copy version 1 into a new draft, add to it, retire
-- version 1 and publish version 2.
-- ===========================================================================

INSERT INTO item_definitions (item_code, display_name, category, stack_limit, trade_policy) VALUES
    ('hat_straw',        'Mũ rơm (mẫu)',             'fashion', 1, 'unbound'),
    ('hat_cap_blue',     'Mũ lưỡi trai xanh (mẫu)',  'fashion', 1, 'unbound'),
    ('hat_flower',       'Vòng hoa (mẫu)',           'fashion', 1, 'bound'),
    ('top_tee_red',      'Áo phông đỏ (mẫu)',        'fashion', 1, 'unbound'),
    ('top_tee_yellow',   'Áo phông vàng (mẫu)',      'fashion', 1, 'unbound'),
    ('top_hoodie_green', 'Áo hoodie xanh lá (mẫu)',  'fashion', 1, 'unbound'),
    ('bottom_jeans',     'Quần jean (mẫu)',          'fashion', 1, 'unbound'),
    ('bottom_shorts',    'Quần short kaki (mẫu)',    'fashion', 1, 'unbound'),
    ('bottom_skirt',     'Váy tím (mẫu)',            'fashion', 1, 'unbound');

INSERT INTO fashion_items (item_code, equip_slot, style) VALUES
    ('hat_straw',        'hat',    '{"hat": "straw"}'),
    ('hat_cap_blue',     'hat',    '{"hat": "cap", "color": "#1c7ed6"}'),
    ('hat_flower',       'hat',    '{"hat": "flower", "color": "#f783ac"}'),
    ('top_tee_red',      'top',    '{"shirt": "#fa5252", "shirt_dark": "#c92a2a"}'),
    ('top_tee_yellow',   'top',    '{"shirt": "#ffd43b", "shirt_dark": "#f08c00"}'),
    ('top_hoodie_green', 'top',    '{"shirt": "#51cf66", "shirt_dark": "#2b8a3e"}'),
    ('bottom_jeans',     'bottom', '{"pants": "#1864ab"}'),
    ('bottom_shorts',    'bottom', '{"pants": "#c9a26b"}'),
    ('bottom_skirt',     'bottom', '{"pants": "#9c36b5"}');

INSERT INTO game_config_versions (id, label, state, notes)
VALUES (2, 'DEV-SAMPLE-2', 'draft', 'DEV SAMPLE: v1 + sample fashion offers and daily quests');
SELECT setval('game_config_versions_id_seq', 2);

INSERT INTO game_config_values (config_version_id, key, value)
SELECT 2, key, value FROM game_config_values WHERE config_version_id = 1;
INSERT INTO game_config_values (config_version_id, key, value) VALUES
    (2, 'quests.daily', '{"timezone": "Asia/Ho_Chi_Minh"}');

INSERT INTO crop_configs (config_version_id, crop_code, growth_seconds, base_yield, pest_probability)
SELECT 2, crop_code, growth_seconds, base_yield, pest_probability FROM crop_configs WHERE config_version_id = 1;

INSERT INTO npc_offers (config_version_id, npc_code, item_code, direction, currency, unit_price, sort_order)
SELECT 2, npc_code, item_code, direction, currency, unit_price, sort_order FROM npc_offers WHERE config_version_id = 1;
INSERT INTO npc_offers (config_version_id, npc_code, item_code, direction, currency, unit_price, sort_order) VALUES
    (2, 'FASHION_SHOP', 'hat_straw',        'NPC_SELLS', 'XU',       300, 1),
    (2, 'FASHION_SHOP', 'hat_cap_blue',     'NPC_SELLS', 'XU',       600, 2),
    (2, 'FASHION_SHOP', 'hat_flower',       'NPC_SELLS', 'DIAMOND',    5, 3),
    (2, 'FASHION_SHOP', 'top_tee_red',      'NPC_SELLS', 'XU',       800, 4),
    (2, 'FASHION_SHOP', 'top_tee_yellow',   'NPC_SELLS', 'XU',       800, 5),
    (2, 'FASHION_SHOP', 'top_hoodie_green', 'NPC_SELLS', 'XU',      1500, 6),
    (2, 'FASHION_SHOP', 'bottom_jeans',     'NPC_SELLS', 'XU',       900, 7),
    (2, 'FASHION_SHOP', 'bottom_shorts',    'NPC_SELLS', 'XU',       700, 8),
    (2, 'FASHION_SHOP', 'bottom_skirt',     'NPC_SELLS', 'XU',       900, 9);

INSERT INTO quest_definitions (config_version_id, quest_code, title, event_type, target_quantity, reward_currency, reward_amount, sort_order) VALUES
    (2, 'daily_plant',   'Gieo 3 ô đất',               'plant_crop',       3, 'XU',      200, 1),
    (2, 'daily_harvest', 'Thu hoạch 3 ô đất',          'harvest_crop',     3, 'XU',      300, 2),
    (2, 'daily_sell',    'Bán 20 nông sản cho NPC',    'sell_crop_to_npc', 20, 'XU',     300, 3),
    (2, 'daily_pest',    'Bắt 1 con sâu',              'catch_pest',       1, 'DIAMOND',   1, 4),
    (2, 'daily_chat',    'Chào mọi người trong phòng', 'public_chat',      1, 'XU',      100, 5);

UPDATE game_config_versions SET state = 'retired' WHERE id = 1;
UPDATE game_config_versions SET state = 'published', published_at = now() WHERE id = 2;
