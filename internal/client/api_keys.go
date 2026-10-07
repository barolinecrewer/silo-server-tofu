package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// API key endpoints. Contract (live reference,
// <server>/api/v2/openapi.json): paths /api/v2/admin/api-keys{,/{id}},
// /api/v2/admin/api-keys/{id}/tier, and /api/v2/admin/api-keys/capabilities —
// create (POST -> 201, key disclosed once), read (GET -> 200 + ETag), tier
// update (PUT /tier, If-Match required -> 200 + ETag), delete (DELETE,
// If-Match required -> 204), cursor-paginated list. Only the rate tier is
// updatable; label, scopes, and user are set at create only.
const (
	pathAPIKeys            = "/admin/api-keys"
	pathAPIKeyFmt          = "/admin/api-keys/%s"
	pathAPIKeyTierFmt      = "/admin/api-keys/%s/tier"
	pathAPIKeyCapabilities = "/admin/api-keys/capabilities"
)

// APIKeyCreate mirrors the AdminAPIKeyCreateInputBody request schema. Fields
// are pointers: nil means "omit on create" (the server default applies — an
// omitted user_id means the authenticated account).
type APIKeyCreate struct {
	Label  string    `json:"label"`
	Scopes *[]string `json:"scopes,omitempty"`
	UserID *string   `json:"user_id,omitempty"`
}

// APIKeyCreated mirrors the AdminAPIKeyCreated response: the only place the
// server ever discloses the full key.
type APIKeyCreated struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Label     string   `json:"label"`
	Key       string   `json:"key"`
	RateTier  string   `json:"rate_tier"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
}

// APIKey mirrors the AdminAPIKey response schema. Later reads see only the
// key prefix, never the key itself.
type APIKey struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Label     string   `json:"label"`
	KeyPrefix string   `json:"key_prefix"`
	RateTier  string   `json:"rate_tier"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
}

// APIKeyTierUpdate mirrors the AdminAPIKeyTierInputBody request schema.
type APIKeyTierUpdate struct {
	RateTier string `json:"rate_tier"`
}

// APIKeyListItem mirrors AdminAPIKeyListItem: the key plus the list-only
// username and last_used_at. last_used_at is a live value and is never
// pinned in resource state.
type APIKeyListItem struct {
	APIKey
	Username   string  `json:"username"`
	LastUsedAt *string `json:"last_used_at"`
}

// APIKeyCapabilities mirrors AdminAPIKeyCapabilitiesOutputBody.
type APIKeyCapabilities struct {
	State                string   `json:"state"`
	Allowed              bool     `json:"allowed"`
	Available            bool     `json:"available"`
	GuardedConfiguration bool     `json:"guarded_configuration"`
	RateTiers            []string `json:"rate_tiers"`
	Revision             string   `json:"revision"`
}

type apiKeyListResponse struct {
	Items []APIKeyListItem `json:"items"`
	Page  *PageInfo        `json:"page"`
}

// GetAPIKeyCapabilities reads the domain capability document.
func (c *Client) GetAPIKeyCapabilities(ctx context.Context) (*APIKeyCapabilities, error) {
	var out APIKeyCapabilities
	_, err := c.do(ctx, http.MethodGet, pathAPIKeyCapabilities, nil, nil, &out, nil)
	return &out, err
}

// CreateAPIKey posts a new key and returns the one-time created payload.
// The 201 response carries no ETag (only Location); read the key to get one.
func (c *Client) CreateAPIKey(ctx context.Context, write *APIKeyCreate) (*APIKeyCreated, error) {
	var out APIKeyCreated
	resp, err := c.do(ctx, http.MethodPost, pathAPIKeys, nil, write, &out, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d creating api key", resp.StatusCode)
	}
	return &out, nil
}

// GetAPIKey reads one key and returns its current ETag.
func (c *Client) GetAPIKey(ctx context.Context, id string) (*APIKey, string, error) {
	var out APIKey
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf(pathAPIKeyFmt, url.PathEscape(id)), nil, nil, &out, nil)
	if err != nil {
		return nil, "", err
	}
	return &out, resp.ETag, nil
}

// UpdateAPIKeyTier sets the key's rate tier. etag must be the value from the
// last read (state); a stale etag returns a 412 APIError.
func (c *Client) UpdateAPIKeyTier(ctx context.Context, id, etag, rateTier string) (*APIKey, string, error) {
	var out APIKey
	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf(pathAPIKeyTierFmt, url.PathEscape(id)), nil, &APIKeyTierUpdate{RateTier: rateTier}, &out, ifMatch(etag))
	if err != nil {
		return nil, "", err
	}
	return &out, resp.ETag, nil
}

// DeleteAPIKey removes a key. A 404 is returned as nil (already gone);
// other errors are APIErrors.
func (c *Client) DeleteAPIKey(ctx context.Context, id, etag string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf(pathAPIKeyFmt, url.PathEscape(id)), nil, nil, nil, ifMatch(etag))
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d deleting api key %s", resp.StatusCode, id)
	}
	return nil
}

// ListAPIKeys reads one page of keys. limit is clamped to the API's
// documented 1..200 range; cursor is opaque and comes from a prior call.
func (c *Client) ListAPIKeys(ctx context.Context, cursor string, limit int) ([]APIKeyListItem, PageInfo, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}

	var out apiKeyListResponse
	if _, err := c.do(ctx, http.MethodGet, pathAPIKeys, query, nil, &out, nil); err != nil {
		return nil, PageInfo{}, err
	}
	page := PageInfo{}
	if out.Page != nil {
		page = *out.Page
	}
	// The contract promises items is present and never null.
	if out.Items == nil {
		out.Items = []APIKeyListItem{}
	}
	return out.Items, page, nil
}

// ListAllAPIKeys walks every page of the collection. Use for data sources;
// do not assume page ordering or count pages.
func (c *Client) ListAllAPIKeys(ctx context.Context) ([]APIKeyListItem, error) {
	var all []APIKeyListItem
	cursor := ""
	for {
		items, page, err := c.ListAPIKeys(ctx, cursor, maxPageLimit)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if !page.HasMore {
			return all, nil
		}
		if page.NextCursor == "" {
			return nil, fmt.Errorf("api key list has_more without next_cursor")
		}
		cursor = page.NextCursor
	}
}
