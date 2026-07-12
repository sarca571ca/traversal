package esi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/antihax/goesi"
	"golang.org/x/oauth2"
)

const eveTokenURL = "https://login.eveonline.com/v2/oauth/token"

type Client struct {
	api        *goesi.APIClient
	httpClient *http.Client
	clientID   string
}

func New(httpClient *http.Client, userAgent, clientID string) *Client {
	return &Client{
		api:        goesi.NewAPIClient(httpClient, userAgent),
		httpClient: httpClient,
		clientID:   clientID,
	}
}

type SystemInfo struct {
	ID       int32
	Name     string
	Security float32
}

func (c *Client) GetSystemInfo(ctx context.Context, systemID int32) (*SystemInfo, error) {
	info, _, err := c.api.ESI.UniverseApi.GetUniverseSystemsSystemId(ctx, systemID, nil)
	if err != nil {
		return nil, fmt.Errorf("get system %d: %w", systemID, err)
	}
	return &SystemInfo{
		ID:       systemID,
		Name:     info.Name,
		Security: info.SecurityStatus,
	}, nil
}

// GetSystemStatics fetches the static wormhole type codes for a J-space system.
// Uses a raw HTTP call because the goesi library's generated struct omits the statics field.
func (c *Client) GetSystemStatics(ctx context.Context, systemID int32) ([]string, error) {
	rawURL := fmt.Sprintf("https://esi.evetech.net/v4/universe/systems/%d/", systemID)
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get system statics %d: %w", systemID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get system statics %d: status %d", systemID, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Statics []string `json:"statics"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode system statics %d: %w", systemID, err)
	}
	return result.Statics, nil
}

// GetCharacterLocation fetches the current solar system for a character.
// Returns the system ID and the current token (which may have been refreshed).
func (c *Client) GetCharacterLocation(ctx context.Context, characterID int64, accessToken, refreshToken string, expiry time.Time) (int32, *oauth2.Token, error) {
	cfg := &oauth2.Config{
		ClientID: c.clientID,
		Endpoint: oauth2.Endpoint{TokenURL: eveTokenURL},
	}
	ts := cfg.TokenSource(ctx, &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiry,
	})

	// Calling Token() here triggers a proactive refresh if the token is
	// expired or close to expiry, giving us back the new tokens to persist.
	tok, err := ts.Token()
	if err != nil {
		return 0, nil, fmt.Errorf("token refresh for character %d: %w", characterID, err)
	}

	authCtx := context.WithValue(ctx, goesi.ContextOAuth2, oauth2.StaticTokenSource(tok))
	loc, _, err := c.api.ESI.LocationApi.GetCharactersCharacterIdLocation(authCtx, int32(characterID), nil)
	if err != nil {
		return 0, nil, fmt.Errorf("get location for character %d: %w", characterID, err)
	}
	return loc.SolarSystemId, tok, nil
}

// SearchSystems resolves a batch of system names to IDs in a single ESI call.
// Names not found by ESI are omitted from the result map.
func (c *Client) SearchSystems(ctx context.Context, names []string) (map[string]int32, error) {
	result, _, err := c.api.ESI.UniverseApi.PostUniverseIds(ctx, names, nil)
	if err != nil {
		return nil, fmt.Errorf("search systems: %w", err)
	}
	m := make(map[string]int32, len(result.Systems))
	for _, s := range result.Systems {
		m[s.Name] = s.Id
	}
	return m, nil
}

// SearchSystem resolves a system name to its ID.
func (c *Client) SearchSystem(ctx context.Context, name string) (int32, error) {
	result, _, err := c.api.ESI.UniverseApi.PostUniverseIds(ctx, []string{name}, nil)
	if err != nil {
		return 0, fmt.Errorf("search system %q: %w", name, err)
	}
	if len(result.Systems) == 0 {
		return 0, fmt.Errorf("system %q not found", name)
	}
	return result.Systems[0].Id, nil
}

// GetRoute returns an ordered list of system IDs from origin to destination.
// connections is a list of extra [fromID, toID] pairs (wormholes).
// flag is "shortest", "secure", or "insecure".
//
// Uses a direct HTTP call because goesi's parameterToString mangles [][]int32
// with multiple pairs, leaving bracket characters in the query string.
func (c *Client) GetRoute(ctx context.Context, originID, destID int32, connections [][2]int32, flag string) ([]int32, error) {
	// ESI treats connections as a flat array of int32 and groups consecutive
	// integers into pairs. Each ID must be a separate &connections= entry.
	rawURL := fmt.Sprintf("https://esi.evetech.net/v1/route/%d/%d/?flag=%s",
		originID, destID, url.QueryEscape(flag))
	for _, p := range connections {
		rawURL += fmt.Sprintf("&connections=%d&connections=%d", p[0], p[1])
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("route %d→%d: %w", originID, destID, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no route found between the two systems (try enabling wormhole shortcuts)")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("route %d→%d: %s %s", originID, destID, resp.Status, string(body))
	}

	var route []int32
	if err := json.Unmarshal(body, &route); err != nil {
		return nil, fmt.Errorf("route decode: %w", err)
	}
	return route, nil
}

// GetRegionConstellations returns the constellation IDs belonging to a region.
func (c *Client) GetRegionConstellations(ctx context.Context, regionID int32) ([]int32, error) {
	r, _, err := c.api.ESI.UniverseApi.GetUniverseRegionsRegionId(ctx, regionID, nil)
	if err != nil {
		return nil, fmt.Errorf("get region %d: %w", regionID, err)
	}
	return r.Constellations, nil
}

// GetConstellationInfo returns the constellation name and system IDs it contains.
func (c *Client) GetConstellationInfo(ctx context.Context, constellationID int32) (name string, systems []int32, err error) {
	r, _, err := c.api.ESI.UniverseApi.GetUniverseConstellationsConstellationId(ctx, constellationID, nil)
	if err != nil {
		return "", nil, fmt.Errorf("get constellation %d: %w", constellationID, err)
	}
	return r.Name, r.Systems, nil
}

// ResolveNames resolves a batch of system IDs to names.
func (c *Client) ResolveNames(ctx context.Context, ids []int32) (map[int32]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	results, _, err := c.api.ESI.UniverseApi.PostUniverseNames(ctx, ids, nil)
	if err != nil {
		return nil, fmt.Errorf("resolve names: %w", err)
	}
	names := make(map[int32]string, len(results))
	for _, r := range results {
		names[r.Id] = r.Name
	}
	return names, nil
}
