# PROMPT TASK — World / Farm / Inventory

Đọc `docs/02_GAMEPLAY_FARM_INVENTORY_ECONOMY.md`, `docs/05_TECHNICAL_ARCHITECTURE.md`, `docs/08_WEBSOCKET_PROTOCOL.md`, `docs/10_ACCEPTANCE_TESTS.md` và sổ P0.

- World: server Go map collision/Tiled phiên bản, room auto capacity, WS JSON+binary, movement input authoritative, prediction client, 30s reconnect, friend join room; test W-01..08.
- Wallet/Inventory: fixed starting Xu=5000 KC=0, cap config, farm/upgrade stack<=999 fashion 1 per instance/slot, idempotency transaction and ledger; test T-01..05.
- Farm: 5 cây, seed reference 100/250/500/750/1000 Xu, growth=30/60/120/180/240m, yield/sell/pest probabilities Admin config; không fake permanent numbers; event sâu persisted once, spawn<=70% growth, deadline=30% full growth, penalty floor 5..10%; test F-01..17.
- Whole-batch pending: 1 crop yields >= free capacity => **whole crop into pending**, merge crop type FIFO; claim whole row except P0-083 special; no direct NPC sale from pending.
- NPC: trái menu/phải pane, config giá sau bảo trì, không đứng chồng nhau/collision.
- Hệ chim, nạp QR, chợ player **không thuộc MVP**.

Triển khai theo PR nhỏ, chạy tests, ghi rõ blockers chưa có giá bán/năng suất/xác suất sâu.
