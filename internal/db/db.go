package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS characters (
    id                INTEGER PRIMARY KEY,
    name              TEXT    NOT NULL,
    access_token      TEXT    NOT NULL,
    refresh_token     TEXT    NOT NULL,
    token_expiry      INTEGER NOT NULL,
    current_system_id INTEGER
);

CREATE TABLE IF NOT EXISTS systems (
    id       INTEGER PRIMARY KEY,
    name     TEXT    NOT NULL,
    security REAL    NOT NULL DEFAULT 0,
    x        REAL    NOT NULL DEFAULT 0,
    y        REAL    NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS character_skills (
    character_id         INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id             INTEGER NOT NULL,
    active_skill_level   INTEGER NOT NULL,
    trained_skill_level  INTEGER NOT NULL,
    skillpoints_in_skill INTEGER NOT NULL,
    updated_at           INTEGER NOT NULL,
    PRIMARY KEY (character_id, skill_id)
);

CREATE TABLE IF NOT EXISTS character_implants (
    character_id INTEGER NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    type_id      INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (character_id, type_id)
);

CREATE TABLE IF NOT EXISTS signatures (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    solar_system_id INTEGER NOT NULL,
    sig_id          TEXT    NOT NULL,
    sig_type        TEXT,
    wormhole_type   TEXT,
    leads_to_system INTEGER,
    leads_to_sig    TEXT,
    mass_status     TEXT,
    eol             INTEGER NOT NULL DEFAULT 0,
    expires_at      INTEGER,
    notes           TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    UNIQUE (solar_system_id, sig_id)
);

CREATE TABLE IF NOT EXISTS connections (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    system_a    INTEGER NOT NULL,
    system_b    INTEGER NOT NULL,
    wh_type_a   TEXT    NOT NULL DEFAULT '',
    wh_type_b   TEXT    NOT NULL DEFAULT '',
    sig_id_a    TEXT    NOT NULL DEFAULT '',
    sig_id_b    TEXT    NOT NULL DEFAULT '',
    ship_size   TEXT    NOT NULL DEFAULT '',
    mass_status TEXT    NOT NULL DEFAULT 'stable',
    eol         INTEGER NOT NULL DEFAULT 0,
    expires_at  INTEGER,
    created_at  INTEGER NOT NULL,
    UNIQUE(system_a, system_b)
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

PRAGMA foreign_keys = ON;
`

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, err
	}
	// Best-effort migrations for existing databases — all errors silently ignored.
	sqlDB.Exec(`ALTER TABLE connections RENAME COLUMN wh_type TO wh_type_a`)
	sqlDB.Exec(`ALTER TABLE connections RENAME COLUMN sig_id  TO sig_id_a`)
	sqlDB.Exec(`ALTER TABLE connections ADD COLUMN wh_type_b TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE connections ADD COLUMN sig_id_b  TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE connections ADD COLUMN ship_size TEXT NOT NULL DEFAULT ''`)
	sqlDB.Exec(`ALTER TABLE connections ADD COLUMN expires_at INTEGER`)
	// Map position of an unknown-destination WH stub, relative to its system.
	sqlDB.Exec(`ALTER TABLE signatures ADD COLUMN stub_dx REAL`)
	sqlDB.Exec(`ALTER TABLE signatures ADD COLUMN stub_dy REAL`)
	return &DB{sql: sqlDB}, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}
