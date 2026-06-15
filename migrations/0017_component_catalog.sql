CREATE TABLE component_categories (
    id              TEXT        PRIMARY KEY,
    org_id          UUID        REFERENCES orgs(id) ON DELETE CASCADE,
    kind            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    slug            TEXT        NOT NULL,
    sort_order      INT         NOT NULL DEFAULT 0,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX component_categories_system_slug
    ON component_categories (kind, slug) WHERE org_id IS NULL;
CREATE UNIQUE INDEX component_categories_org_slug
    ON component_categories (org_id, kind, slug) WHERE org_id IS NOT NULL;
CREATE INDEX component_categories_kind ON component_categories(kind);

CREATE TABLE components (
    id              TEXT        PRIMARY KEY,
    org_id          UUID        REFERENCES orgs(id) ON DELETE CASCADE,
    kind            TEXT        NOT NULL,
    type            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    slug            TEXT        NOT NULL,
    description     TEXT        NOT NULL DEFAULT '',
    category_id     TEXT        NOT NULL REFERENCES component_categories(id),
    tags            JSONB       NOT NULL DEFAULT '[]',
    icon_key        TEXT,
    is_active       BOOLEAN     NOT NULL DEFAULT TRUE,
    sort_order      INT         NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX components_system_slug
    ON components (kind, slug) WHERE org_id IS NULL;
CREATE UNIQUE INDEX components_org_slug
    ON components (org_id, kind, slug) WHERE org_id IS NOT NULL;
CREATE INDEX components_kind ON components(kind);
CREATE INDEX components_category_id ON components(category_id);

CREATE TABLE component_fields (
    id              TEXT        PRIMARY KEY,
    component_id    TEXT        NOT NULL REFERENCES components(id) ON DELETE CASCADE,
    label           TEXT        NOT NULL,
    type            TEXT        NOT NULL,
    required        BOOLEAN     NOT NULL DEFAULT FALSE,
    readonly        BOOLEAN,
    options         JSONB,
    sort_order      INT         NOT NULL DEFAULT 0
);

CREATE INDEX component_fields_component_id ON component_fields(component_id);
