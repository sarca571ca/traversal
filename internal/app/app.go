package app

import (
	"context"
	"log"
	"traversal/internal/auth"
	"traversal/internal/cache"
	"traversal/internal/config"
	"traversal/internal/db"
	"traversal/internal/esi"
)

type Character struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	CurrentSystem string `json:"currentSystem"`
}

type App struct {
	ctx   context.Context
	ca    *cache.Cache
	cfg   config.Config
	store *db.DB
	esi   *esi.Client
}

func New(cfg config.Config) (*App, error) {
	c := cache.NewDefault()
	store, err := db.Open("traversal.db")
	if err != nil {
		return nil, err
	}

	return &App{
		ca:    c,
		cfg:   cfg,
		store: store,
		esi:   esi.New(c.Client(), cfg.Name, cfg.ClientID),
	}, nil
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	go a.pollLocations()
	go a.SeedPochven()
}

func (a *App) AddCharacter() error {
	result, err := auth.Login(a.cfg)
	if err != nil {
		return err
	}

	return a.store.UpsertCharacter(db.Character{
		ID:           result.CharacterID,
		Name:         result.CharacterName,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenExpiry:  result.Expiry,
	})
}

func (a *App) GetCharacters() []Character {
	chars, err := a.store.ListCharacters()
	if err != nil {
		log.Println("GetCharacters:", err)
		return nil
	}
	result := make([]Character, len(chars))
	for i, c := range chars {
		sysName := ""
		if c.CurrentSystemID != 0 {
			if sys, _ := a.store.GetSystem(c.CurrentSystemID); sys != nil {
				sysName = sys.Name
			}
		}
		result[i] = Character{ID: c.ID, Name: c.Name, CurrentSystem: sysName}
	}
	return result
}

func (a *App) RemoveCharacter(id int64) error {
	return a.store.DeleteCharacter(id)
}

func dbSystemFromInfo(info *esi.SystemInfo) db.System {
	return db.System{
		ID:       info.ID,
		Name:     info.Name,
		Security: info.Security,
	}
}
