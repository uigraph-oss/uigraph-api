-- Manually registered models can now be edited and deleted from ML Studio.
-- Deletion is soft, so record who did it the same way 0070 did for experiments.

ALTER TABLE ml_models ADD COLUMN deleted_by UUID;
