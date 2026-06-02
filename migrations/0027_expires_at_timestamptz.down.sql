ALTER TABLE forms
    ALTER COLUMN expires_at TYPE DATE USING expires_at::DATE;
