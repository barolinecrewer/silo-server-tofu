package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Contract: /api/v2/admin/users in the live Silo API v2 reference.
const (
	pathAccounts            = "/admin/users"
	pathAccountFmt          = "/admin/users/%s"
	pathAccountCapabilities = "/admin/users/capabilities"
)

// Account contains the configurable account fields and response-only metadata.
// Pointer fields preserve the API's null (inherit) semantics.
type Account struct {
	ID                         string    `json:"id"`
	Username                   string    `json:"username"`
	Email                      string    `json:"email"`
	Role                       string    `json:"role"`
	Enabled                    bool      `json:"enabled"`
	Permissions                []string  `json:"permissions"`
	AccessGroupID              *string   `json:"access_group_id"`
	LibraryIDs                 *[]string `json:"library_ids"`
	MaxPlaybackQuality         *string   `json:"max_playback_quality"`
	MaxStreams                 *int64    `json:"max_streams"`
	MaxTranscodes              *int64    `json:"max_transcodes"`
	MaxRemoteStreamBitrateKbps *int64    `json:"max_remote_stream_bitrate_kbps"`
	MaxLocalStreamBitrateKbps  *int64    `json:"max_local_stream_bitrate_kbps"`
	TranscodeAllowed           *bool     `json:"transcode_allowed"`
	AudioTranscodeAllowed      *bool     `json:"audio_transcode_allowed"`
	DownloadAllowed            *bool     `json:"download_allowed"`
	DownloadTranscodeAllowed   *bool     `json:"download_transcode_allowed"`
	RequestsAllowed            *bool     `json:"requests_allowed"`
	MaxProfiles                int64     `json:"max_profiles"`
	PasswordLogin              bool      `json:"password_login"`
	PasswordChangeRequired     bool      `json:"password_change_required"`
	IsOwner                    bool      `json:"is_owner"`
	CreatedAt                  string    `json:"created_at"`
	UpdatedAt                  string    `json:"updated_at"`
	LastActiveAt               *string   `json:"last_active_at"`
}

type AccountCapabilities struct {
	State                string `json:"state"`
	Allowed              bool   `json:"allowed"`
	Available            bool   `json:"available"`
	AccessGroups         bool   `json:"access_groups"`
	DefaultProfile       bool   `json:"default_profile"`
	GuardedConfiguration bool   `json:"guarded_configuration"`
}

type accountCreated struct {
	ID string `json:"id"`
}
type accountListResponse struct {
	Items []Account `json:"items"`
	Page  *PageInfo `json:"page"`
}

func (c *Client) GetAccountCapabilities(ctx context.Context) (*AccountCapabilities, error) {
	var out AccountCapabilities
	_, err := c.do(ctx, http.MethodGet, pathAccountCapabilities, nil, nil, &out, nil)
	return &out, err
}

func (c *Client) CreateAccount(ctx context.Context, body any) (string, error) {
	var out accountCreated
	resp, err := c.do(ctx, http.MethodPost, pathAccounts, nil, body, &out, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated || out.ID == "" {
		return "", fmt.Errorf("unexpected create account response (status %d, id %q)", resp.StatusCode, out.ID)
	}
	return out.ID, nil
}

func (c *Client) GetAccount(ctx context.Context, id string) (*Account, string, error) {
	var out Account
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf(pathAccountFmt, url.PathEscape(id)), nil, nil, &out, nil)
	if err != nil {
		return nil, "", err
	}
	return &out, resp.ETag, nil
}

func (c *Client) UpdateAccount(ctx context.Context, id, etag string, body any) error {
	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf(pathAccountFmt, url.PathEscape(id)), nil, body, nil, ifMatch(etag))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d updating account", resp.StatusCode)
	}
	return nil
}

func (c *Client) DeleteAccount(ctx context.Context, id, etag string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf(pathAccountFmt, url.PathEscape(id)), nil, nil, nil, ifMatch(etag))
	if IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d deleting account", resp.StatusCode)
	}
	return nil
}

func (c *Client) ListAllAccounts(ctx context.Context) ([]Account, error) {
	var all []Account
	cursor := ""
	for {
		query := url.Values{"limit": {strconv.Itoa(maxPageLimit)}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		var out accountListResponse
		if _, err := c.do(ctx, http.MethodGet, pathAccounts, query, nil, &out, nil); err != nil {
			return nil, err
		}
		all = append(all, out.Items...)
		if out.Page == nil || !out.Page.HasMore {
			return all, nil
		}
		if out.Page.NextCursor == "" {
			return nil, fmt.Errorf("account list has_more without next_cursor")
		}
		cursor = out.Page.NextCursor
	}
}
