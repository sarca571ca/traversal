package app

import "traversal/internal/db"

type SigEntry struct {
	ID      int64  `json:"id"`
	SigID   string `json:"sigId"`
	SigType string `json:"sigType"`
	Notes   string `json:"notes"`
}

func (a *App) GetSignatures(systemID int32) []SigEntry {
	sigs, err := a.store.ListSignaturesBySystem(systemID)
	if err != nil {
		return nil
	}
	result := make([]SigEntry, len(sigs))
	for i, s := range sigs {
		result[i] = SigEntry{
			ID:      s.ID,
			SigID:   s.SigID,
			SigType: string(s.SigType),
			Notes:   s.Notes,
		}
	}
	return result
}

func (a *App) SaveSignature(systemID int32, sigID, sigType, notes string) error {
	_, err := a.store.UpsertSignature(db.Signature{
		SolarSystemID: systemID,
		SigID:         sigID,
		SigType:       db.SigType(sigType),
		Notes:         notes,
	})
	return err
}

func (a *App) DeleteSignature(id int64) error {
	return a.store.DeleteSignature(id)
}

func (a *App) ClearSignatures(systemID int32) error {
	return a.store.DeleteSignaturesBySystem(systemID)
}
