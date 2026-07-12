package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"traversal/internal/config"

	jwt "github.com/golang-jwt/jwt/v4"
)

const (
	eveAuthURL  = "https://login.eveonline.com/v2/oauth/authorize"
	eveTokenURL = "https://login.eveonline.com/v2/oauth/token"
)

type TokenResult struct {
	CharacterID   int64
	CharacterName string
	AccessToken   string
	RefreshToken  string
	Expiry        time.Time
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type eveClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

func generateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func buildAuthURL(cfg config.Config, state, challenge string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {cfg.CallbackURL},
		"scope":                 {strings.Join(cfg.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return eveAuthURL + "?" + params.Encode()
}

func postToken(values url.Values) (*tokenResponse, error) {
	resp, err := http.PostForm(eveTokenURL, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func parseCharacterFromToken(accessToken string) (int64, string, time.Time, error) {
	var claims eveClaims
	p := new(jwt.Parser)
	_, _, err := p.ParseUnverified(accessToken, &claims)
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("parse jwt: %w", err)
	}

	parts := strings.SplitN(claims.Subject, ":", 3)
	if len(parts) != 3 {
		return 0, "", time.Time{}, fmt.Errorf("unexpected sub format: %s", claims.Subject)
	}

	charID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("parse character id: %w", err)
	}

	var expiry time.Time
	if claims.ExpiresAt != nil {
		expiry = claims.ExpiresAt.Time
	}

	return charID, claims.Name, expiry, nil
}

func buildTokenResult(tr *tokenResponse) (*TokenResult, error) {
	charID, charName, expiry, err := parseCharacterFromToken(tr.AccessToken)
	if err != nil {
		return nil, err
	}
	return &TokenResult{
		CharacterID:   charID,
		CharacterName: charName,
		AccessToken:   tr.AccessToken,
		RefreshToken:  tr.RefreshToken,
		Expiry:        expiry,
	}, nil
}

func Login(cfg config.Config) (*TokenResult, error) {
	verifier, err := generateVerifier()
	if err != nil {
		return nil, err
	}

	challenge := generateChallenge(verifier)

	state, err := generateState()
	if err != nil {
		return nil, err
	}

	authURL := buildAuthURL(cfg, state, challenge)

	ch, srv, err := startCallbackServer(state)
	if err != nil {
		return nil, err
	}
	go func() { _ = srv.ListenAndServeTLS("", "") }()

	if err := exec.Command("xdg-open", authURL).Start(); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	result := <-ch
	if result.err != nil {
		return nil, result.err
	}

	tr, err := postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {result.code},
		"client_id":     {cfg.ClientID},
		"code_verifier": {verifier},
		"redirect_uri":  {cfg.CallbackURL},
	})
	if err != nil {
		return nil, err
	}

	return buildTokenResult(tr)
}

func Refresh(cfg config.Config, refreshToken string) (*TokenResult, error) {
	tr, err := postToken(url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.ClientID},
	})
	if err != nil {
		return nil, err
	}

	return buildTokenResult(tr)
}
