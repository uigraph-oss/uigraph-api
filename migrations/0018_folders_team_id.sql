ALTER TABLE folders
    ADD COLUMN team_id UUID REFERENCES teams(id) ON DELETE SET NULL;

CREATE INDEX idx_folders_team ON folders(team_id) WHERE deleted_at IS NULL;
