INSERT INTO llm_models (id, model_id, provider, display_name, input_cost_per_million, output_cost_per_million) VALUES
    ('06', 'gpt-5.5',  'openai',  'GPT-5.5',  5.00, 30.00),
    ('07', 'glm-5.2',  'zhipuai', 'GLM-5.2',  1.10,  3.85);

UPDATE llm_models SET input_cost_per_million = 6.00, output_cost_per_million = 30.00, updated_at = NOW()
    WHERE model_id = 'claude-opus-4-8';
