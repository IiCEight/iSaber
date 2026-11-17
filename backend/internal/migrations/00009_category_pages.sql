-- +goose Up
ALTER TABLE categories ADD COLUMN description_en text NOT NULL DEFAULT '';
ALTER TABLE categories ADD COLUMN description_zh text NOT NULL DEFAULT '';
ALTER TABLE categories ADD CONSTRAINT category_description_lengths CHECK (
    char_length(description_en)<=2000 AND char_length(description_zh)<=2000);
ALTER TABLE articles ADD COLUMN category_pinned boolean NOT NULL DEFAULT false;
ALTER TABLE articles ADD CONSTRAINT category_pin_requires_publication CHECK (
    NOT category_pinned OR published_revision_id IS NOT NULL);

-- Also protects rollbacks to binaries unaware of category pins.
-- +goose StatementBegin
CREATE FUNCTION maintain_category_pin() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE new_category uuid; old_category uuid;
BEGIN
    IF NEW.published_revision_id IS NULL AND NEW.category_pinned AND
        (TG_OP='INSERT' OR OLD.published_revision_id IS NULL) THEN
        RAISE EXCEPTION 'category pin requires publication' USING ERRCODE='23514';
    END IF;
    SELECT category_id INTO new_category FROM revisions WHERE id=NEW.published_revision_id;
    IF TG_OP='UPDATE' AND NEW.published_revision_id IS DISTINCT FROM OLD.published_revision_id THEN
        SELECT category_id INTO old_category FROM revisions WHERE id=OLD.published_revision_id;
        IF new_category IS DISTINCT FROM old_category THEN NEW.category_pinned := false; END IF;
    END IF;
    IF NEW.published_revision_id IS NULL THEN NEW.category_pinned := false; END IF;
    IF NEW.category_pinned AND new_category IS NULL THEN
        RAISE EXCEPTION 'category pin requires a published category' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER maintain_category_pin BEFORE INSERT OR UPDATE OF published_revision_id,category_pinned
    ON articles FOR EACH ROW EXECUTE FUNCTION maintain_category_pin();
CREATE INDEX articles_category_pinned_idx ON articles(category_pinned DESC, published_at DESC, id DESC)
    WHERE published_revision_id IS NOT NULL;
