package api

import "context"

// DataCenter describes one of RunPod's physical datacenters and the GPU SKUs
// currently surfaced there. IDs look like "CA-MTL-3", "EU-CZ-1", "US-IL-1".
//
// The GpuAvailability slice is the inverse view of `gpu list`: instead of
// "what does this GPU cost?" it answers "where is this GPU?". RunPod returns
// nothing useful for `GpuAvailability` over REST — this lives on the public
// GraphQL endpoint, like `gpu list`.
type DataCenter struct {
	ID              string            `json:"id"`
	Name            string            `json:"name,omitempty"`
	Location        string            `json:"location,omitempty"`
	StorageSupport  bool              `json:"storageSupport"`
	Listed          bool              `json:"listed"`
	GpuAvailability []GpuAvailability `json:"gpuAvailability,omitempty"`
}

// GpuAvailability is a single GPU SKU surfaced inside a datacenter.
// StockStatus is "Low" or null. Available is whether RunPod will currently
// accept a pod-create against that SKU in that DC.
type GpuAvailability struct {
	GpuTypeID   string `json:"gpuTypeId"`
	DisplayName string `json:"displayName,omitempty"`
	StockStatus string `json:"stockStatus,omitempty"`
	Available   bool   `json:"available"`
}

const dataCentersQuery = `query DataCenters {
  dataCenters {
    id
    name
    location
    storageSupport
    listed
    gpuAvailability {
      gpuTypeId
      displayName
      stockStatus
      available
    }
  }
}`

// ListDataCenters returns every datacenter RunPod exposes, along with its
// per-DC GPU availability. Uses GraphQL because the REST API does not expose
// datacenters at all. Works without an API key.
func (c *Client) ListDataCenters(ctx context.Context) ([]DataCenter, error) {
	var resp struct {
		DataCenters []DataCenter `json:"dataCenters"`
	}
	if err := c.GraphQL(ctx, dataCentersQuery, nil, &resp); err != nil {
		return nil, err
	}
	return resp.DataCenters, nil
}
