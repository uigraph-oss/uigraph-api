UPDATE ml_models SET considerations = ethical_considerations
WHERE considerations = '' AND ethical_considerations <> '';

UPDATE ml_models SET recommendations = caveats
WHERE recommendations = '' AND caveats <> '';

ALTER TABLE ml_models DROP COLUMN ethical_considerations;
ALTER TABLE ml_models DROP COLUMN caveats;
