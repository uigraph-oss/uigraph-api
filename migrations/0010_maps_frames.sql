-- Maps: top-level UI journey containers (renamed from "projects").
-- Frames: individual screens/pages within a map (renamed from "pages").
-- Focal points: named clickable hotspots on a frame.
-- Map canvas: persisted pan/zoom + frame positions for the map board view.

CREATE TABLE maps (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    folder_id   UUID        REFERENCES folders(id) ON DELETE SET NULL,
    team_id     UUID        REFERENCES teams(id) ON DELETE SET NULL,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID
);

CREATE INDEX idx_maps_org ON maps(org_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_maps_folder ON maps(folder_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_maps_team ON maps(team_id) WHERE deleted_at IS NULL;

-- Frames belong to a map and optionally nest under a parent frame.
CREATE TABLE frames (
    id                      UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    map_id                  UUID        NOT NULL REFERENCES maps(id) ON DELETE CASCADE,
    org_id                  UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    parent_frame_id         UUID        REFERENCES frames(id) ON DELETE CASCADE,
    name                    TEXT        NOT NULL,
    description             TEXT        NOT NULL DEFAULT '',
    template_type           TEXT        NOT NULL DEFAULT '',
    screenshot_key          TEXT,       -- object storage key for frame screenshot
    screenshot_content_hash TEXT,       -- SHA-256 of screenshot; used by CLI sync to skip unchanged
    status                  TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    ord                     DOUBLE PRECISION NOT NULL DEFAULT 0,
    source                  TEXT,       -- 'cli' | 'github' | NULL (UI)
    created_by              UUID        NOT NULL,
    updated_by              UUID,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at              TIMESTAMPTZ,
    deleted_by              UUID
);

CREATE INDEX idx_frames_map ON frames(map_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_frames_parent ON frames(parent_frame_id) WHERE deleted_at IS NULL;

-- Focal points: named hotspots pinned to a (x,y) location on a frame.
CREATE TABLE focal_points (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    frame_id    UUID        NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    location_x  DOUBLE PRECISION NOT NULL DEFAULT 0,
    location_y  DOUBLE PRECISION NOT NULL DEFAULT 0,
    visibility  TEXT        NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','private')),
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID
);

CREATE INDEX idx_focal_points_frame ON focal_points(frame_id) WHERE deleted_at IS NULL;

-- Map canvas: pan/zoom state + per-frame positions for the board view.
-- One row per map; upserted on every canvas save.
CREATE TABLE map_canvas (
    map_id          UUID        NOT NULL REFERENCES maps(id) ON DELETE CASCADE PRIMARY KEY,
    org_id          UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    zoom            DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    navigation_x    DOUBLE PRECISION NOT NULL DEFAULT 0,
    navigation_y    DOUBLE PRECISION NOT NULL DEFAULT 0,
    frame_positions JSONB       NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
