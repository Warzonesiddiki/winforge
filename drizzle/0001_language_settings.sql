-- Migration: ensure app_settings.language exists (idempotent)
-- Phase 5 Localization: 5-language scaffolding (en-US, es-ES, fr-FR, de-DE, zh-CN)
-- No DB wipe — additive only, default en-US, fallback to en.

-- Create enum type if not exists (Drizzle may manage this separately)
DO $$ BEGIN
  -- app_settings table already exists from initial migration;
  -- this migration ensures the language column is present.
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'app_settings' AND column_name = 'language'
  ) THEN
    ALTER TABLE app_settings ADD COLUMN language TEXT NOT NULL DEFAULT 'en-US';
  END IF;
END $$;

-- Backfill any NULL or empty values (idempotent)
UPDATE app_settings SET language = 'en-US' WHERE language IS NULL OR language = '';

-- Settings are singleton row id=1; ensure default row respects language.
INSERT INTO app_settings (id, language)
VALUES (1, 'en-US')
ON CONFLICT (id) DO NOTHING;
