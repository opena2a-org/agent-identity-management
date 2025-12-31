-- A2A Intent-Based Discovery: Add full-text search to skills
-- Migration 066: Enable intent-based agent routing via PostgreSQL FTS

-- ============================================================================
-- Add search vector to a2a_skills for intent-based routing
-- ============================================================================

-- Add tsvector column for full-text search
ALTER TABLE a2a_skills ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Create function to generate search vector from skill data
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
            (SELECT string_agg(tag::text, ' ')
             FROM jsonb_array_elements_text(skill_tags) AS tag),
            ''
        )
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Populate search_vector for existing rows
UPDATE a2a_skills
SET search_vector = a2a_skill_search_vector(name, description, tags)
WHERE search_vector IS NULL;

-- Create trigger to auto-update search_vector
CREATE OR REPLACE FUNCTION update_a2a_skill_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector = a2a_skill_search_vector(NEW.name, NEW.description, NEW.tags);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_a2a_skills_search_vector ON a2a_skills;
CREATE TRIGGER trigger_a2a_skills_search_vector
    BEFORE INSERT OR UPDATE OF name, description, tags ON a2a_skills
    FOR EACH ROW EXECUTE FUNCTION update_a2a_skill_search_vector();

-- Create GIN index for fast full-text search
CREATE INDEX IF NOT EXISTS idx_a2a_skills_search ON a2a_skills USING GIN(search_vector);

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON COLUMN a2a_skills.search_vector IS 'Full-text search vector for intent-based agent discovery';
