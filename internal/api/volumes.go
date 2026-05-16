package api

import (
	"context"
	"net/url"
)

// NetworkVolume is a persistent NFS-backed volume that can be mounted into pods.
type NetworkVolume struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	Size         int    `json:"size,omitempty"`
	DataCenterID string `json:"dataCenterId,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
}

// ListVolumes returns every network volume on the account.
func (c *Client) ListVolumes(ctx context.Context) ([]NetworkVolume, error) {
	var vols []NetworkVolume
	if err := c.Do(ctx, "GET", "/networkvolumes", nil, &vols); err != nil {
		return nil, err
	}
	return vols, nil
}

// GetVolume returns a single network volume by ID.
func (c *Client) GetVolume(ctx context.Context, id string) (*NetworkVolume, error) {
	var v NetworkVolume
	if err := c.Do(ctx, "GET", "/networkvolumes/"+url.PathEscape(id), nil, &v); err != nil {
		return nil, err
	}
	return &v, nil
}
