-- Rollback: Remove standard tags (mark as non-standard)
UPDATE tags SET is_standard = false, display_order = NULL WHERE is_standard = true;
