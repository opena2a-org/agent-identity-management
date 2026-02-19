-- Fix: Handle NULL or non-array tags in a2a_skill_search_vector function
-- The original function from migration 067 calls jsonb_array_elements_text()
-- without checking if skill_tags is actually a JSON array, causing
-- "cannot extract elements from a scalar" errors on skill creation.

CREATE OR REPLACE FUNCTION a2a_skill_search_vector(
    skill_name VARCHAR,
    skill_description TEXT,
    skill_tags JSONB
) RETURNS tsvector AS $$
BEGIN
    RETURN to_tsvector('english',
        COALESCE(skill_name, '') || ' ' ||
        COALESCE(skill_description, '') || ' ' ||
        COALESCE(
            CASE
                WHEN skill_tags IS NOT NULL AND jsonb_typeof(skill_tags) = 'array'
                THEN (SELECT string_agg(tag::text, ' ')
                      FROM jsonb_array_elements_text(skill_tags) AS tag)
                ELSE NULL
            END,
            ''
        )
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;
