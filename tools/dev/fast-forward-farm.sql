-- LOCAL DEVELOPMENT ONLY: moves every growing planting (and its pest) back in
-- time so growth, pests and harvest can be tested without waiting.
-- It does not touch assets or the ledger. Never run it on a shared database.
--
--   docker compose -f deploy/compose.local.yml exec -T postgres \
--     psql -U thanh_dev -d thanh_dev -v minutes=31 -f - < tools/dev/fast-forward-farm.sql
BEGIN;
UPDATE pest_incidents i
   SET spawn_at = spawn_at - make_interval(mins => :minutes),
       deadline_at = deadline_at - make_interval(mins => :minutes),
       caught_at = caught_at - make_interval(mins => :minutes)
  FROM crop_plantings p
 WHERE p.id = i.planting_id AND p.state = 'growing';
UPDATE crop_plantings
   SET planted_at = planted_at - make_interval(mins => :minutes),
       matures_at = matures_at - make_interval(mins => :minutes)
 WHERE state = 'growing';
COMMIT;
