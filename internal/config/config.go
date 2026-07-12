package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Name        string
	ClientID    string
	CallbackURL string
	Scopes      []string
}

var scopes = []string{
	"esi-location.read_location.v1",
	"esi-location.read_ship_type.v1",
	"esi-skills.read_skills.v1",
	"esi-fittings.read_fittings.v1",
	"esi-clones.read_implants.v1",
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		Name:        envOr("NAME", defaultName),
		ClientID:    envOr("CLIENTID", defaultClientID),
		CallbackURL: envOr("CALLBACKURL", defaultCallbackURL),
		Scopes:      scopes,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
