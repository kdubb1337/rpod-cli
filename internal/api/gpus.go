package api

import "context"

// GPUType describes one of RunPod's available GPU SKUs.
// IDs look like "NVIDIA A100 80GB PCIe", "NVIDIA GeForce RTX 4090", etc. — pass
// them verbatim to PodCreateRequest.GPUTypeIDs.
type GPUType struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"displayName,omitempty"`
	MemoryInGb      int     `json:"memoryInGb,omitempty"`
	SecureCloud     bool    `json:"secureCloud,omitempty"`
	CommunityCloud  bool    `json:"communityCloud,omitempty"`
	SecurePrice     float64 `json:"securePrice,omitempty"`
	CommunityPrice  float64 `json:"communityPrice,omitempty"`
	OneMonthPrice   float64 `json:"oneMonthPrice,omitempty"`
	ThreeMonthPrice float64 `json:"threeMonthPrice,omitempty"`
	Manufacturer    string  `json:"manufacturer,omitempty"`
}

const gpuTypesQuery = `query GpuTypes {
  gpuTypes {
    id
    displayName
    memoryInGb
    secureCloud
    communityCloud
    securePrice
    communityPrice
    oneMonthPrice
    threeMonthPrice
    manufacturer
  }
}`

// ListGPUTypes returns every GPU type RunPod offers. Uses GraphQL because the
// REST API does not expose gpu types.
func (c *Client) ListGPUTypes(ctx context.Context) ([]GPUType, error) {
	var resp struct {
		GPUTypes []GPUType `json:"gpuTypes"`
	}
	if err := c.GraphQL(ctx, gpuTypesQuery, nil, &resp); err != nil {
		return nil, err
	}
	return resp.GPUTypes, nil
}
