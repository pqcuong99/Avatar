-- ===========================================================================
-- DEV SAMPLE CONFIG — LOCAL DEVELOPMENT ONLY. NOT PRODUCTION VALUES.
--
-- Applied by `migrate` only when THANH_ENV=local. Every number below that the
-- docs mark [CẦN CẤU HÌNH] / ADMIN_TODO (yield, NPC buy-back prices, pest
-- probability, Diamond cap, upgrade prices and limits, room capacity, chat
-- limits, NPC names) is an invented placeholder so the core loop can be played
-- locally. The Admin must publish real values through maintenance before any
-- Closed Beta (GAP-04, GAP-05, GAP-07, GAP-11, GAP-13, GAP-15).
--
-- Values that ARE decided and copied as-is: growth minutes 30/60/120/180/240
-- (P0-061), reference seed prices 100/250/500/750/1000 Xu (P0-060, still to
-- be re-confirmed by the Admin before launch), Xu wallet cap 999,999,999
-- (P0-035).
-- ===========================================================================

INSERT INTO npc_definitions (npc_code, display_name, functions) VALUES
    ('FARM_SHOP',    'Bác Nông (mẫu)',  ARRAY['buy_seeds', 'sell_crops']),
    ('FASHION_SHOP', 'Cô Thợ May (mẫu)', ARRAY['buy_fashion']);

INSERT INTO game_config_versions (id, label, state, notes)
VALUES (1, 'DEV-SAMPLE-1', 'draft', 'DEV SAMPLE: placeholder numbers for local development only');
SELECT setval('game_config_versions_id_seq', 1);

INSERT INTO game_config_values (config_version_id, key, value) VALUES
    (1, 'wallet.xu_cap',        '999999999'),
    (1, 'wallet.diamond_cap',   '100000'),
    (1, 'inventory.upgrade',    '{"currency": "XU", "price": 1000, "slots_per_upgrade": 5, "max_slot_cap": 30}'),
    (1, 'farm.plot_unlock',     '{"currency": "XU", "price": 2000, "max_plots": 12}'),
    (1, 'world.room_capacity',  '30'),
    (1, 'chat.public',          '{"max_chars": 200, "min_interval_ms": 800}');

INSERT INTO crop_configs (config_version_id, crop_code, growth_seconds, base_yield, pest_probability) VALUES
    (1, 'carrot',   30 * 60, 10, 0.300000),
    (1, 'potato',   60 * 60, 12, 0.300000),
    (1, 'tomato',  120 * 60, 14, 0.300000),
    (1, 'corn',    180 * 60, 16, 0.300000),
    (1, 'cabbage', 240 * 60, 18, 0.300000);

INSERT INTO npc_offers (config_version_id, npc_code, item_code, direction, currency, unit_price, sort_order) VALUES
    (1, 'FARM_SHOP', 'seed_carrot',  'NPC_SELLS', 'XU',  100, 1),
    (1, 'FARM_SHOP', 'seed_potato',  'NPC_SELLS', 'XU',  250, 2),
    (1, 'FARM_SHOP', 'seed_tomato',  'NPC_SELLS', 'XU',  500, 3),
    (1, 'FARM_SHOP', 'seed_corn',    'NPC_SELLS', 'XU',  750, 4),
    (1, 'FARM_SHOP', 'seed_cabbage', 'NPC_SELLS', 'XU', 1000, 5),
    (1, 'FARM_SHOP', 'crop_carrot',  'NPC_BUYS',  'XU',   15, 1),
    (1, 'FARM_SHOP', 'crop_potato',  'NPC_BUYS',  'XU',   30, 2),
    (1, 'FARM_SHOP', 'crop_tomato',  'NPC_BUYS',  'XU',   50, 3),
    (1, 'FARM_SHOP', 'crop_corn',    'NPC_BUYS',  'XU',   65, 4),
    (1, 'FARM_SHOP', 'crop_cabbage', 'NPC_BUYS',  'XU',   80, 5);

UPDATE game_config_versions SET state = 'published', published_at = now() WHERE id = 1;
