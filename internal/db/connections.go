package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Connection struct {
	ID         int64
	SystemA    int32
	SystemB    int32
	WHTypeA    string
	WHTypeB    string
	SigIDA     string
	SigIDB     string
	ShipSize   string
	MassStatus string
	EOL        bool
	ExpiresAt  int64 // Unix timestamp; 0 = no expiry
	CreatedAt  time.Time
}

// UpsertConnection records a connection between two systems, smaller ID first.
func (db *DB) UpsertConnection(id1, id2 int32) error {
	a, b := id1, id2
	if a > b {
		a, b = b, a
	}
	_, err := db.sql.Exec(`
		INSERT INTO connections (system_a, system_b, mass_status, eol, created_at)
		VALUES (?, ?, 'stable', 0, ?)
		ON CONFLICT(system_a, system_b) DO NOTHING
	`, a, b, time.Now().Unix())
	return err
}

func (db *DB) GetAllConnections() ([]Connection, error) {
	rows, err := db.sql.Query(`
		SELECT id, system_a, system_b,
		       COALESCE(wh_type_a,''), COALESCE(wh_type_b,''),
		       COALESCE(sig_id_a,''),  COALESCE(sig_id_b,''),
		       COALESCE(ship_size,''), COALESCE(mass_status,'stable'),
		       eol, COALESCE(expires_at,0), created_at
		FROM connections
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conns []Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		conns = append(conns, *c)
	}
	return conns, rows.Err()
}

// UpdateConnection saves WH info. timerHours: -1 = keep existing, 0 = clear, 1/4/24 = set new.
func (db *DB) UpdateConnection(id int64, whTypeA, whTypeB, sigIdA, sigIdB, shipSize, massStatus string, eol bool, timerHours int) error {
	if timerHours == -1 {
		_, err := db.sql.Exec(`
			UPDATE connections
			SET wh_type_a=?, wh_type_b=?, sig_id_a=?, sig_id_b=?,
			    ship_size=?, mass_status=?, eol=?
			WHERE id=?
		`, whTypeA, whTypeB, sigIdA, sigIdB, shipSize, massStatus, eol, id)
		return err
	}
	var expiresAt interface{}
	if timerHours > 0 {
		expiresAt = time.Now().Add(time.Duration(timerHours) * time.Hour).Unix()
	}
	_, err := db.sql.Exec(`
		UPDATE connections
		SET wh_type_a=?, wh_type_b=?, sig_id_a=?, sig_id_b=?,
		    ship_size=?, mass_status=?, eol=?, expires_at=?
		WHERE id=?
	`, whTypeA, whTypeB, sigIdA, sigIdB, shipSize, massStatus, eol, expiresAt, id)
	return err
}

// GetExpiredConnections returns connections whose timer has elapsed (but does not delete them).
func (db *DB) GetExpiredConnections() ([]Connection, error) {
	rows, err := db.sql.Query(`
		SELECT id, system_a, system_b,
		       COALESCE(wh_type_a,''), COALESCE(wh_type_b,''),
		       COALESCE(sig_id_a,''),  COALESCE(sig_id_b,''),
		       COALESCE(ship_size,''), COALESCE(mass_status,'stable'),
		       eol, COALESCE(expires_at,0), created_at
		FROM connections WHERE expires_at IS NOT NULL AND expires_at <= ?
	`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conns []Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		conns = append(conns, *c)
	}
	return conns, rows.Err()
}

// DeleteExpiredConnections removes connections whose timer has elapsed.
// Returns the number of rows deleted.
func (db *DB) DeleteExpiredConnections() (int64, error) {
	res, err := db.sql.Exec(`
		DELETE FROM connections WHERE expires_at IS NOT NULL AND expires_at <= ?
	`, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (db *DB) DeleteConnection(id int64) error {
	_, err := db.sql.Exec(`DELETE FROM connections WHERE id = ?`, id)
	return err
}

func (db *DB) DeleteConnectionsExcept(keepIDs []int32) error {
	if len(keepIDs) == 0 {
		_, err := db.sql.Exec(`DELETE FROM connections`)
		return err
	}
	ph := make([]string, len(keepIDs))
	args := make([]interface{}, len(keepIDs)*2)
	for i, id := range keepIDs {
		ph[i] = "?"
		args[i] = id
		args[len(keepIDs)+i] = id
	}
	inClause := strings.Join(ph, ",")
	_, err := db.sql.Exec(
		fmt.Sprintf(`DELETE FROM connections WHERE system_a NOT IN (%[1]s) OR system_b NOT IN (%[1]s)`, inClause),
		args...,
	)
	return err
}

func scanConnection(s scanner) (*Connection, error) {
	var c Connection
	var eol int
	var createdAt int64
	if err := s.Scan(
		&c.ID, &c.SystemA, &c.SystemB,
		&c.WHTypeA, &c.WHTypeB, &c.SigIDA, &c.SigIDB,
		&c.ShipSize, &c.MassStatus, &eol, &c.ExpiresAt, &createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	c.EOL = eol == 1
	c.CreatedAt = time.Unix(createdAt, 0)
	return &c, nil
}

// GetConnectionBetween returns the connection joining two systems (either order), or nil.
func (db *DB) GetConnectionBetween(id1, id2 int32) (*Connection, error) {
	a, b := id1, id2
	if a > b {
		a, b = b, a
	}
	row := db.sql.QueryRow(`
		SELECT id, system_a, system_b,
		       COALESCE(wh_type_a,''), COALESCE(wh_type_b,''),
		       COALESCE(sig_id_a,''),  COALESCE(sig_id_b,''),
		       COALESCE(ship_size,''), COALESCE(mass_status,'stable'),
		       eol, COALESCE(expires_at,0), created_at
		FROM connections WHERE system_a = ? AND system_b = ?
	`, a, b)
	return scanConnection(row)
}

// SetConnectionSigs updates only the sig IDs and, when expiresAt > 0, the expiry.
// Everything else (WH types, mass, ship size, EOL) is left untouched.
func (db *DB) SetConnectionSigs(id int64, sigIDA, sigIDB string, expiresAt int64) error {
	if expiresAt > 0 {
		_, err := db.sql.Exec(`UPDATE connections SET sig_id_a=?, sig_id_b=?, expires_at=? WHERE id=?`,
			sigIDA, sigIDB, expiresAt, id)
		return err
	}
	_, err := db.sql.Exec(`UPDATE connections SET sig_id_a=?, sig_id_b=? WHERE id=?`, sigIDA, sigIDB, id)
	return err
}
