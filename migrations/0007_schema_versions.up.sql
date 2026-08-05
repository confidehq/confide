CREATE TABLE IF NOT EXISTS form_schema_versions (
  form_id          TEXT     NOT NULL REFERENCES forms(id) ON DELETE CASCADE,
  version          INTEGER  NOT NULL,
  encrypted_schema BYTEA    NOT NULL,
  created_at       DATE     NOT NULL,
  PRIMARY KEY (form_id, version)
);
