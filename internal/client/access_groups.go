package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Access group endpoints. Contract (live reference,
// <server>/api/v2/openapi.json): paths /api/v2/admin/access-groups{,/{id}} —
// create (POST -> 201), read (GET -> 200 + ETag), full-replace update
// (PUT, If-Match required), delete (DELETE, If-Match required -> 204),
// cursor-paginated list.
const (
	pathAccessGroups   = "/admin/access-groups"
	pathAccessGroupFmt = "/admin/access-groups/%s"
)

// AccessGroupWrite mirrors the AdminAccessGroupBody request schema. Fields
// are pointers: nil means "omit on create" (server default applies), and
// update bodies carry the full desired state. nullable schema fields use
// pointer-to-slice so an explicitly empty list is sent as [] rather than
// omitted.
type AccessGroupWrite struct {
	Name                     string    `json:"name"`
	Description              *string   `json:"description,omitempty"`
	LibraryIDs               *[]string `json:"library_ids,omitempty"`
	MaxPlaybackQuality       *string   `json:"max_playback_quality,omitempty"`
	DownloadAllowed          *bool     `json:"download_allowed,omitempty"`
	DownloadTranscodeAllowed *bool     `json:"download_transcode_allowed,omitempty"`
	TranscodeAllowed         *bool     `json:"transcode_allowed,omitempty"`
	AudioTranscodeAllowed    *bool     `json:"audio_transcode_allowed,omitempty"`
	MaxStreams               *int64    `json:"max_streams,omitempty"`
	MaxTranscodes            *int64    `json:"max_transcodes,omitempty"`
	MaxRemoteBitrateKbps     *int64    `json:"max_remote_stream_bitrate_kbps,omitempty"`
	MaxLocalBitrateKbps      *int64    `json:"max_local_stream_bitrate_kbps,omitempty"`
	AllowedPermissions       *[]string `json:"allowed_permissions,omitempty"`
	RequestsAllowed          *bool     `json:"requests_allowed,omitempty"`
	IsDefault                *bool     `json:"is_default,omitempty"`
}

// AccessGroup mirrors the AdminAccessGroup response schema. IDs are opaque
// strings; timestamps are RFC 3339 with millisecond precision.
type AccessGroup struct {
	ID                       string   `json:"id"`
	Name                     string   `json:"name"`
	Description              string   `json:"description"`
	LibraryIDs               []string `json:"library_ids"`
	MaxPlaybackQuality       string   `json:"max_playback_quality"`
	DownloadAllowed          bool     `json:"download_allowed"`
	DownloadTranscodeAllowed bool     `json:"download_transcode_allowed"`
	TranscodeAllowed         bool     `json:"transcode_allowed"`
	AudioTranscodeAllowed    bool     `json:"audio_transcode_allowed"`
	MaxStreams               int64    `json:"max_streams"`
	MaxTranscodes            int64    `json:"max_transcodes"`
	MaxRemoteBitrateKbps     int64    `json:"max_remote_stream_bitrate_kbps"`
	MaxLocalBitrateKbps      int64    `json:"max_local_stream_bitrate_kbps"`
	AllowedPermissions       []string `json:"allowed_permissions"`
	RequestsAllowed          bool     `json:"requests_allowed"`
	IsDefault                bool     `json:"is_default"`
	CreatedAt                string   `json:"created_at"`
	UpdatedAt                string   `json:"updated_at"`
}

// AccessGroupListItem mirrors AdminAccessGroupListItem: the full group plus
// the list-only member_count. The member count is a live value and is never
// pinned in resource state.
type AccessGroupListItem struct {
	AccessGroup
	MemberCount int64 `json:"member_count"`
}

// PageInfo mirrors the shared PageInfo cursor schema. Cursors are opaque:
// pass next_cursor back unchanged, never count pages.
type PageInfo struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

type accessGroupListResponse struct {
	Items []AccessGroupListItem `json:"items"`
	Page  *PageInfo             `json:"page"`
}

// CreateAccessGroup posts a new group and returns it with the response
// ETag for later guarded writes.
func (c *Client) CreateAccessGroup(ctx context.Context, write *AccessGroupWrite) (*AccessGroup, string, error) {
	var out AccessGroup
	resp, err := c.do(ctx, http.MethodPost, pathAccessGroups, nil, write, &out, nil)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, "", fmt.Errorf("unexpected status %d creating access group", resp.StatusCode)
	}
	return &out, resp.ETag, nil
}

// GetAccessGroup reads one group and returns its current ETag.
func (c *Client) GetAccessGroup(ctx context.Context, id string) (*AccessGroup, string, error) {
	var out AccessGroup
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf(pathAccessGroupFmt, id), nil, nil, &out, nil)
	if err != nil {
		return nil, "", err
	}
	return &out, resp.ETag, nil
}

// UpdateAccessGroup full-replaces a group. etag must be the value from the
// last read (state); a stale etag returns a 412 APIError.
func (c *Client) UpdateAccessGroup(ctx context.Context, id, etag string, write *AccessGroupWrite) (*AccessGroup, string, error) {
	var out AccessGroup
	resp, err := c.do(ctx, http.MethodPut, fmt.Sprintf(pathAccessGroupFmt, id), nil, write, &out, ifMatch(etag))
	if err != nil {
		return nil, "", err
	}
	return &out, resp.ETag, nil
}

// DeleteAccessGroup removes a group. A 404 is returned as nil (already
// gone); other errors are APIErrors.
func (c *Client) DeleteAccessGroup(ctx context.Context, id, etag string) error {
	resp, err := c.do(ctx, http.MethodDelete, fmt.Sprintf(pathAccessGroupFmt, id), nil, nil, nil, ifMatch(etag))
	if err != nil {
		if IsNotFound(err) {
			return nil
		}
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d deleting access group %s", resp.StatusCode, id)
	}
	return nil
}

// ListAccessGroups reads one page of groups. limit is clamped to the API's
// documented 1..200 range; cursor is opaque and comes from a prior call.
func (c *Client) ListAccessGroups(ctx context.Context, cursor string, limit int) ([]AccessGroupListItem, PageInfo, error) {
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

	var out accessGroupListResponse
	if _, err := c.do(ctx, http.MethodGet, pathAccessGroups, query, nil, &out, nil); err != nil {
		return nil, PageInfo{}, err
	}
	page := PageInfo{}
	if out.Page != nil {
		page = *out.Page
	}
	// The contract promises items is present and never null.
	if out.Items == nil {
		out.Items = []AccessGroupListItem{}
	}
	return out.Items, page, nil
}

// ListAllAccessGroups walks every page of the collection. Use for data
// sources; do not assume page ordering or count pages.
func (c *Client) ListAllAccessGroups(ctx context.Context) ([]AccessGroupListItem, error) {
	var all []AccessGroupListItem
	cursor := ""
	for {
		items, page, err := c.ListAccessGroups(ctx, cursor, maxPageLimit)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if !page.HasMore || page.NextCursor == "" {
			return all, nil
		}
		cursor = page.NextCursor
	}
}
