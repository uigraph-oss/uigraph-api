ALTER TABLE ml_models ADD COLUMN owners                 TEXT   NOT NULL DEFAULT '';
ALTER TABLE ml_models ADD COLUMN license                TEXT   NOT NULL DEFAULT '';
ALTER TABLE ml_models ADD COLUMN reference_links        TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE ml_models ADD COLUMN intended_use           TEXT   NOT NULL DEFAULT '';
ALTER TABLE ml_models ADD COLUMN limitations            TEXT   NOT NULL DEFAULT '';
ALTER TABLE ml_models ADD COLUMN ethical_considerations TEXT   NOT NULL DEFAULT '';
ALTER TABLE ml_models ADD COLUMN caveats                TEXT   NOT NULL DEFAULT '';
