-- Reference screenshots attached to a test case's own definition (distinct
-- from TestRunResult.screenshot_urls, which captures evidence from
-- *executing* a test case). Stores asset IDs, resolved to signed URLs via
-- the existing asset-url query.

ALTER TABLE test_cases ADD COLUMN screenshot_urls TEXT[];
