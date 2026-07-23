ALTER TABLE ml_projects
    ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

UPDATE ml_projects m
    SET team_id = t.id
    FROM teams t
    WHERE t.org_id = m.org_id AND t.name = m.team;

CREATE INDEX idx_ml_projects_team ON ml_projects(team_id) WHERE deleted_at IS NULL;

ALTER TABLE ml_projects DROP COLUMN team;
ALTER TABLE ml_projects DROP COLUMN email;
