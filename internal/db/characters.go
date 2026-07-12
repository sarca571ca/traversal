package db

import (
	"database/sql"
	"errors"
	"time"
)

type Character struct {
	ID              int64
	Name            string
	AccessToken     string
	RefreshToken    string
	TokenExpiry     time.Time
	CurrentSystemID int32
}

func (db *DB) UpsertCharacter(c Character) error {
	_, err := db.sql.Exec(`
		INSERT INTO characters (id, name, access_token, refresh_token, token_expiry)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name          = excluded.name,
			access_token  = excluded.access_token,
			refresh_token = excluded.refresh_token,
			token_expiry  = excluded.token_expiry
	`, c.ID, c.Name, c.AccessToken, c.RefreshToken, c.TokenExpiry.Unix())
	return err
}

func (db *DB) GetCharacter(id int64) (*Character, error) {
	row := db.sql.QueryRow(
		`SELECT id, name, access_token, refresh_token, token_expiry, current_system_id FROM characters WHERE id = ?`, id,
	)
	return scanCharacter(row)
}

func (db *DB) ListCharacters() ([]Character, error) {
	rows, err := db.sql.Query(
		`SELECT id, name, access_token, refresh_token, token_expiry, current_system_id FROM characters ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chars []Character
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		chars = append(chars, *c)
	}
	return chars, rows.Err()
}

func (db *DB) UpdateTokens(id int64, accessToken, refreshToken string, expiry time.Time) error {
	_, err := db.sql.Exec(`
		UPDATE characters SET access_token = ?, refresh_token = ?, token_expiry = ? WHERE id = ?
	`, accessToken, refreshToken, expiry.Unix(), id)
	return err
}

func (db *DB) DeleteCharacter(id int64) error {
	_, err := db.sql.Exec(`DELETE FROM characters WHERE id = ?`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCharacter(s scanner) (*Character, error) {
	var c Character
	var expiry int64
	var systemID sql.NullInt32
	if err := s.Scan(&c.ID, &c.Name, &c.AccessToken, &c.RefreshToken, &expiry, &systemID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	c.TokenExpiry = time.Unix(expiry, 0)
	if systemID.Valid {
		c.CurrentSystemID = systemID.Int32
	}
	return &c, nil
}
