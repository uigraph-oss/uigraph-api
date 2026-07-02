-- Avatars for principals. Both users and service accounts may carry an avatar
-- image stored in object storage under "assets/<assetId>" (user_<id> / sa_<id>),
-- served to the browser as a presigned URL via the asset resolver. NULL = no
-- avatar (the UI falls back to initials).
ALTER TABLE users            ADD COLUMN avatar_asset_id TEXT;
ALTER TABLE service_accounts ADD COLUMN avatar_asset_id TEXT;
