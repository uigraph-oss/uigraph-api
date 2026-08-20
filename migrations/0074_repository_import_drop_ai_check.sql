UPDATE repository_imports SET status='selected'
WHERE status IN ('checking_ai_configuration','waiting_ai_configuration');

ALTER TABLE repository_imports DROP COLUMN missing_ai_configuration;

ALTER TABLE service_account_tokens
    DROP CONSTRAINT service_account_tokens_service_account_id_name_key;
