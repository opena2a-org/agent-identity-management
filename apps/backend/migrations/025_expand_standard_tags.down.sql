-- Rollback: Remove the 10 additional standard tags (keep only first 10)
UPDATE tags
SET is_standard = false, display_order = NULL
WHERE is_standard = true AND display_order > 10;
