CREATE TABLE llm_models (
    id                      TEXT        NOT NULL PRIMARY KEY,
    model_id                TEXT        NOT NULL UNIQUE,
    provider                TEXT        NOT NULL,
    display_name            TEXT        NOT NULL,
    input_cost_per_million  NUMERIC(10,4) NOT NULL,
    output_cost_per_million NUMERIC(10,4) NOT NULL,
    is_active               BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO llm_models (id, model_id, provider, display_name, input_cost_per_million, output_cost_per_million) VALUES
    ('01', 'claude-sonnet-4-6',  'anthropic', 'Claude Sonnet 4.6', 3.00,  15.00),
    ('02', 'claude-opus-4-8',    'anthropic', 'Claude Opus 4.8',   15.00, 75.00),
    ('03', 'claude-haiku-4-5',   'anthropic', 'Claude Haiku 4.5',  0.80,  4.00),
    ('04', 'gpt-4o',             'openai',    'GPT-4o',            2.50,  10.00),
    ('05', 'cursor-default',     'cursor',    'Cursor (default)',   3.00,  15.00);
