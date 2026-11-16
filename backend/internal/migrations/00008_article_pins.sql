-- +goose Up
ALTER TABLE articles ADD COLUMN pinned boolean NOT NULL DEFAULT false;
ALTER TABLE articles ADD CONSTRAINT article_pin_requires_publication
    CHECK (NOT pinned OR published_revision_id IS NOT NULL);

-- Keep unpublishing compatible with the preceding application during rollout.
-- +goose StatementBegin
CREATE FUNCTION clear_unpublished_article_pin() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.published_revision_id IS NULL THEN
        NEW.pinned := false;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER clear_unpublished_article_pin
    BEFORE UPDATE OF published_revision_id ON articles
    FOR EACH ROW EXECUTE FUNCTION clear_unpublished_article_pin();

CREATE INDEX articles_pinned_reading_idx ON articles(pinned DESC, published_at DESC, id DESC)
    WHERE published_revision_id IS NOT NULL;
