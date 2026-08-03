-- linked_map_node_id was typed as a plain UUID, but the UI Map Node picker
-- (link-ui-map-node.tsx) actually submits a composite "mapId:screenId:
-- focalPointId" reference (three UUIDs joined by colons) identifying a
-- specific focal point within a specific frame within a specific map — not
-- a single UUID. Every insert/update carrying a real selection therefore
-- failed with "invalid input syntax for type uuid". The application layer
-- already treats this column as an opaque string (Go: *string), so this is
-- purely a column-type correction — no other code changes are needed.

ALTER TABLE test_cases ALTER COLUMN linked_map_node_id TYPE TEXT;
