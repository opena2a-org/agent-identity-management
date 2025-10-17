-- Fix API key prefix column length
-- Current: VARCHAR(8) - too small for "aim_live_XXXXXXXX" format  
-- New: VARCHAR(20) - sufficient for current and future prefix formats

ALTER TABLE api_keys
ALTER COLUMN prefix TYPE VARCHAR(20);
