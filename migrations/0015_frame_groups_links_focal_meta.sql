-- Frame groups: named rectangular regions drawn on a frame (renamed from "page_group").
-- Frame links: navigation hotspots from a frame to another frame or to a map
--   (unifies enterprise page↔page and page↔project links via the `kind` column).
-- Focal point meta: rich component metadata attached to a focal point.

CREATE TABLE frame_groups (
    id          UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    frame_id    UUID        NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    org_id      UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    location_x  DOUBLE PRECISION NOT NULL DEFAULT 0,
    location_y  DOUBLE PRECISION NOT NULL DEFAULT 0,
    width       DOUBLE PRECISION NOT NULL DEFAULT 0,
    height      DOUBLE PRECISION NOT NULL DEFAULT 0,
    ord         DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by  UUID        NOT NULL,
    updated_by  UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    deleted_by  UUID
);

CREATE INDEX idx_frame_groups_frame ON frame_groups(frame_id) WHERE deleted_at IS NULL;

-- Frame links: a hotspot on a source frame that navigates to a target frame or map.
-- kind = 'frame' -> target_frame_id set; kind = 'map' -> target_map_id set.
CREATE TABLE frame_links (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    frame_id        UUID        NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    org_id          UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    kind            TEXT        NOT NULL CHECK (kind IN ('frame','map')),
    target_frame_id UUID        REFERENCES frames(id) ON DELETE CASCADE,
    target_map_id   UUID        REFERENCES maps(id) ON DELETE CASCADE,
    label           TEXT        NOT NULL DEFAULT '',
    location_x      DOUBLE PRECISION NOT NULL DEFAULT 0,
    location_y      DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_by      UUID        NOT NULL,
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID
);

CREATE INDEX idx_frame_links_frame ON frame_links(frame_id) WHERE deleted_at IS NULL;

-- Focal point meta: links a focal point to a flow-diagram component plus its
-- captured field values, images, and an optional flow diagram reference.
CREATE TABLE focal_point_meta (
    id                   UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    focal_point_id       UUID        NOT NULL REFERENCES focal_points(id) ON DELETE CASCADE,
    org_id               UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    frame_id             UUID        NOT NULL REFERENCES frames(id) ON DELETE CASCADE,
    component_id         TEXT        NOT NULL DEFAULT '',
    component_link_id    TEXT,
    component_images     JSONB       NOT NULL DEFAULT '[]',
    component_flow_diagram TEXT,
    component_modal_fields JSONB     NOT NULL DEFAULT '[]',
    created_by           UUID        NOT NULL,
    updated_by           UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ,
    deleted_by           UUID
);

CREATE INDEX idx_focal_point_meta_focal_point ON focal_point_meta(focal_point_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_focal_point_meta_frame ON focal_point_meta(frame_id) WHERE deleted_at IS NULL;
