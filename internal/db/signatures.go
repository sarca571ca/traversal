package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SigType string

const (
	SigTypeUnknown  SigType = "unknown"
	SigTypeWormhole SigType = "wormhole"
	SigTypeData     SigType = "data"
	SigTypeRelic    SigType = "relic"
	SigTypeGas      SigType = "gas"
	SigTypeCombat   SigType = "combat"
)

type MassStatus string

const (
	MassStable   MassStatus = "stable"
	MassHalf     MassStatus = "half"
	MassCritical MassStatus = "critical"
)

type Signature struct {
	ID            int64
	SolarSystemID int32
	SigID         string
	SigType       SigType
	WormholeType  string
	LeadsToSystem int32
	LeadsToSig    string
	MassStatus    MassStatus
	EOL           bool
	ExpiresAt     time.Time
	Notes         string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (db *DB) UpsertSignature(s Signature) (int64, error) {
	var expiresAt *int64
	if !s.ExpiresAt.IsZero() {
		v := s.ExpiresAt.Unix()
		expiresAt = &v
	}

	var leadsToSystem *int32
	if s.LeadsToSystem != 0 {
		leadsToSystem = &s.LeadsToSystem
	}

	var leadsToSig *string
	if s.LeadsToSig != "" {
		leadsToSig = &s.LeadsToSig
	}

	now := time.Now().Unix()
	eol := 0
	if s.EOL {
		eol = 1
	}

	res, err := db.sql.Exec(`
		INSERT INTO signatures
			(solar_system_id, sig_id, sig_type, wormhole_type, leads_to_system, leads_to_sig,
			 mass_status, eol, expires_at, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(solar_system_id, sig_id) DO UPDATE SET
			sig_type       = excluded.sig_type,
			wormhole_type  = excluded.wormhole_type,
			leads_to_system = excluded.leads_to_system,
			leads_to_sig   = excluded.leads_to_sig,
			mass_status    = excluded.mass_status,
			eol            = excluded.eol,
			expires_at     = excluded.expires_at,
			notes          = excluded.notes,
			updated_at     = excluded.updated_at
	`,
		s.SolarSystemID, s.SigID, string(s.SigType), nullString(string(s.WormholeType)),
		leadsToSystem, leadsToSig,
		nullString(string(s.MassStatus)), eol, expiresAt, nullString(s.Notes),
		now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetSignature(id int64) (*Signature, error) {
	row := db.sql.QueryRow(
		`SELECT id, solar_system_id, sig_id, sig_type, wormhole_type, leads_to_system, leads_to_sig,
			mass_status, eol, expires_at, notes, created_at, updated_at
		FROM signatures WHERE id = ?`, id,
	)
	return scanSignature(row)
}

func (db *DB) ListSignaturesBySystem(solarSystemID int32) ([]Signature, error) {
	rows, err := db.sql.Query(
		`SELECT id, solar_system_id, sig_id, sig_type, wormhole_type, leads_to_system, leads_to_sig,
			mass_status, eol, expires_at, notes, created_at, updated_at
		FROM signatures WHERE solar_system_id = ? ORDER BY sig_id`, solarSystemID,
	)
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

func (db *DB) DeleteSignature(id int64) error {
	_, err := db.sql.Exec(`DELETE FROM signatures WHERE id = ?`, id)
	return err
}

func (db *DB) DeleteSignaturesBySystem(solarSystemID int32) error {
	_, err := db.sql.Exec(`DELETE FROM signatures WHERE solar_system_id = ?`, solarSystemID)
	return err
}

func (db *DB) DeleteSignaturesExcept(keepSystemIDs []int32) error {
	if len(keepSystemIDs) == 0 {
		_, err := db.sql.Exec(`DELETE FROM signatures`)
		return err
	}
	ph := make([]string, len(keepSystemIDs))
	args := make([]interface{}, len(keepSystemIDs))
	for i, id := range keepSystemIDs {
		ph[i] = "?"
		args[i] = id
	}
	_, err := db.sql.Exec(
		fmt.Sprintf(`DELETE FROM signatures WHERE solar_system_id NOT IN (%s)`, strings.Join(ph, ",")),
		args...,
	)
	return err
}

func scanSignature(s scanner) (*Signature, error) {
	var sig Signature
	var sigType, wormholeType, massStatus, notes sql.NullString
	var leadsToSystem sql.NullInt32
	var leadsToSig sql.NullString
	var eol int
	var expiresAt sql.NullInt64
	var createdAt, updatedAt int64

	err := s.Scan(
		&sig.ID, &sig.SolarSystemID, &sig.SigID,
		&sigType, &wormholeType, &leadsToSystem, &leadsToSig,
		&massStatus, &eol, &expiresAt, &notes,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	sig.SigType = SigType(sigType.String)
	sig.WormholeType = wormholeType.String
	sig.LeadsToSystem = leadsToSystem.Int32
	sig.LeadsToSig = leadsToSig.String
	sig.MassStatus = MassStatus(massStatus.String)
	sig.EOL = eol == 1
	sig.Notes = notes.String
	if expiresAt.Valid {
		sig.ExpiresAt = time.Unix(expiresAt.Int64, 0)
	}
	sig.CreatedAt = time.Unix(createdAt, 0)
	sig.UpdatedAt = time.Unix(updatedAt, 0)

	return &sig, nil
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
