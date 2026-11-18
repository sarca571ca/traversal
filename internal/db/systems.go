package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type System struct {
	ID       int32
	Name     string
	Security float32
	X        float64
	Y        float64
}

func (db *DB) UpsertSystem(s System) error {
	_, err := db.sql.Exec(`
		INSERT INTO systems (id, name, security, x, y)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name     = excluded.name,
			security = excluded.security
	`, s.ID, s.Name, s.Security, s.X, s.Y)
	return err
}

// SeedSystem upserts a system and always overwrites its position.
// Used when pre-seeding map layouts where position is intentional.
func (db *DB) SeedSystem(s System) error {
	_, err := db.sql.Exec(`
		INSERT INTO systems (id, name, security, x, y)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name     = excluded.name,
			security = excluded.security,
			x        = excluded.x,
			y        = excluded.y
	`, s.ID, s.Name, s.Security, s.X, s.Y)
	return err
}

func (db *DB) UpdateSystemPosition(id int32, x, y float64) error {
	_, err := db.sql.Exec(
		`UPDATE systems SET x = ?, y = ? WHERE id = ?`, x, y, id,
	)
	return err
}

func (db *DB) GetSystem(id int32) (*System, error) {
	row := db.sql.QueryRow(
		`SELECT id, name, security, x, y FROM systems WHERE id = ?`, id,
	)
	return scanSystem(row)
}

func (db *DB) GetAllSystems() ([]System, error) {
	rows, err := db.sql.Query(
		`SELECT id, name, security, x, y FROM systems ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var systems []System
	for rows.Next() {
		s, err := scanSystem(rows)
		if err != nil {
			return nil, err
		}
		systems = append(systems, *s)
	}
	return systems, rows.Err()
}

func (db *DB) GetConnections() ([]Signature, error) {
	rows, err := db.sql.Query(`
		SELECT id, solar_system_id, sig_id, sig_type, wormhole_type, leads_to_system, leads_to_sig,
			mass_status, eol, expires_at, notes, created_at, updated_at
		FROM signatures
		WHERE sig_type = 'wormhole' AND leads_to_system IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sigs []Signature
	for rows.Next() {
		s, err := scanSignature(rows)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, *s)
	}
	return sigs, rows.Err()
}

func (db *DB) SetCharacterSystem(characterID int64, systemID int32) error {
	_, err := db.sql.Exec(
		`UPDATE characters SET current_system_id = ? WHERE id = ?`, systemID, characterID,
	)
	return err
}

func (db *DB) GetSystemByName(name string) (*System, error) {
	row := db.sql.QueryRow(`SELECT id, name, security, x, y FROM systems WHERE name = ? COLLATE NOCASE`, name)
	return scanSystem(row)
}

func (db *DB) DeleteSystemsExcept(keepIDs []int32) error {
	if len(keepIDs) == 0 {
		_, err := db.sql.Exec(`DELETE FROM systems`)
		return err
	}
	ph := make([]string, len(keepIDs))
	args := make([]interface{}, len(keepIDs))
	for i, id := range keepIDs {
		ph[i] = "?"
		args[i] = id
	}
	_, err := db.sql.Exec(
		fmt.Sprintf(`DELETE FROM systems WHERE id NOT IN (%s)`, strings.Join(ph, ",")),
		args...,
	)
	return err
}

func scanSystem(s scanner) (*System, error) {
	var sys System
	err := s.Scan(&sys.ID, &sys.Name, &sys.Security, &sys.X, &sys.Y)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sys, nil
}
