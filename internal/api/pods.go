package api

import (
	"context"
	"fmt"
	"net/url"
)

// Pod is the RunPod pod resource. Fields chosen for high-signal agent output;
// less-used fields are kept for `raw` rendering.
type Pod struct {
	ID                string            `json:"id"`
	Name              string            `json:"name,omitempty"`
	DesiredStatus     string            `json:"desiredStatus,omitempty"`
	LastStatusChange  string            `json:"lastStatusChange,omitempty"`
	Image             string            `json:"image,omitempty"`
	ImageName         string            `json:"imageName,omitempty"`
	MachineID         string            `json:"machineId,omitempty"`
	GPUTypeID         string            `json:"gpuTypeId,omitempty"`
	GPUCount          int               `json:"gpuCount,omitempty"`
	VCPUCount         int               `json:"vcpuCount,omitempty"`
	MemoryInGb        float64           `json:"memoryInGb,omitempty"`
	ContainerDiskInGb int               `json:"containerDiskInGb,omitempty"`
	VolumeInGb        int               `json:"volumeInGb,omitempty"`
	VolumeMountPath   string            `json:"volumeMountPath,omitempty"`
	NetworkVolumeID   string            `json:"networkVolumeId,omitempty"`
	CostPerHr         float64           `json:"costPerHr,omitempty"`
	Locked            bool              `json:"locked,omitempty"`
	Ports             []string          `json:"ports,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	PublicIP          string            `json:"publicIp,omitempty"`
	CreatedAt         string            `json:"createdAt,omitempty"`
	// PortMappings is the REST-only `{"<privatePort>": <publicPort>}` map that
	// RunPod returns for UI-created / resumed pods. For those pods the GraphQL
	// `runtime.ports` array is empty even though direct TCP routing is live,
	// so SSH/endpoint discovery has to fall back to this field. Combined with
	// PublicIP and the spec Ports list it carries the same info runtime.ports
	// would, just in a different shape.
	PortMappings map[string]int `json:"portMappings,omitempty"`
	// Runtime is the live network surface, populated once the pod is running.
	// Empty while the pod is queued or starting; used by `pod ssh-info`,
	// `pod wait`, `pod url`, `pod exec`, `pod cp`.
	Runtime *PodRuntime `json:"runtime,omitempty"`
}

// PodRuntime mirrors the `runtime` block RunPod attaches to a running pod.
type PodRuntime struct {
	UptimeInSeconds int            `json:"uptimeInSeconds,omitempty"`
	Ports           []PodPort      `json:"ports,omitempty"`
	GPUs            []PodGPUStatus `json:"gpus,omitempty"`
	Container       *PodContainer  `json:"container,omitempty"`
}

// PodPort describes one exposed port on a running pod. `privatePort` is the
// in-container port; `publicPort` is what RunPod maps it to externally when
// `isIpPublic` is true. For HTTP ports without a public IP, RunPod still
// reverse-proxies them at https://<podID>-<privatePort>.proxy.runpod.net.
type PodPort struct {
	IP          string `json:"ip,omitempty"`
	IsIPPublic  bool   `json:"isIpPublic,omitempty"`
	PrivatePort int    `json:"privatePort,omitempty"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Type        string `json:"type,omitempty"` // "http" or "tcp"
}

// PodGPUStatus is the per-GPU runtime telemetry RunPod attaches.
type PodGPUStatus struct {
	ID            string  `json:"id,omitempty"`
	GPUUtilPct    float64 `json:"gpuUtilPercent,omitempty"`
	MemoryUtilPct float64 `json:"memoryUtilPercent,omitempty"`
}

// PodContainer is the per-container runtime telemetry RunPod attaches.
type PodContainer struct {
	CPUPct    float64 `json:"cpuPercent,omitempty"`
	MemoryPct float64 `json:"memoryPercent,omitempty"`
}

// CreatePodRequest mirrors the REST API's POST /pods PodCreateInput body. We
// expose only the commonly-used subset; extra fields can be added as needed.
type CreatePodRequest struct {
	Name              string            `json:"name,omitempty"`
	ImageName         string            `json:"imageName,omitempty"`
	ComputeType       string            `json:"computeType,omitempty"`
	GPUTypeIDs        []string          `json:"gpuTypeIds,omitempty"`
	GPUCount          int               `json:"gpuCount,omitempty"`
	CloudType         string            `json:"cloudType,omitempty"`
	ContainerDiskInGb int               `json:"containerDiskInGb,omitempty"`
	VolumeInGb        int               `json:"volumeInGb,omitempty"`
	VolumeMountPath   string            `json:"volumeMountPath,omitempty"`
	NetworkVolumeID   string            `json:"networkVolumeId,omitempty"`
	Ports             []string          `json:"ports,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	DataCenterIDs     []string          `json:"dataCenterIds,omitempty"`
	SupportPublicIP   bool              `json:"supportPublicIp,omitempty"`
	MinVCPUPerGPU     int               `json:"minVCPUPerGPU,omitempty"`
	MinRAMPerGPU      int               `json:"minRAMPerGPU,omitempty"`
	TemplateID        string            `json:"templateId,omitempty"`
	Interruptible     bool              `json:"interruptible,omitempty"`
}

// ListPodsOptions narrows the list endpoint. RunPod's REST list returns all
// pods on the account; we keep the surface small.
type ListPodsOptions struct {
	DesiredStatus string // RUNNING, EXITED, etc.
}

// ListPods returns every pod on the account, optionally filtered locally.
func (c *Client) ListPods(ctx context.Context, opts ListPodsOptions) ([]Pod, error) {
	var pods []Pod
	if err := c.Do(ctx, "GET", "/pods", nil, &pods); err != nil {
		return nil, err
	}
	if opts.DesiredStatus != "" {
		filtered := pods[:0]
		for _, p := range pods {
			if p.DesiredStatus == opts.DesiredStatus {
				filtered = append(filtered, p)
			}
		}
		pods = filtered
	}
	return pods, nil
}

// GetPod returns a single pod by ID.
func (c *Client) GetPod(ctx context.Context, id string) (*Pod, error) {
	var pod Pod
	if err := c.Do(ctx, "GET", "/pods/"+url.PathEscape(id), nil, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// CreatePod creates a new pod.
func (c *Client) CreatePod(ctx context.Context, req CreatePodRequest) (*Pod, error) {
	var pod Pod
	if err := c.Do(ctx, "POST", "/pods", req, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// DeletePod deletes a pod by ID.
func (c *Client) DeletePod(ctx context.Context, id string) error {
	return c.Do(ctx, "DELETE", "/pods/"+url.PathEscape(id), nil, nil)
}

// StartPod resumes a stopped pod.
func (c *Client) StartPod(ctx context.Context, id string) (*Pod, error) {
	var pod Pod
	if err := c.Do(ctx, "POST", fmt.Sprintf("/pods/%s/start", url.PathEscape(id)), nil, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// StopPod halts a running pod (preserving its persistent volume).
func (c *Client) StopPod(ctx context.Context, id string) (*Pod, error) {
	var pod Pod
	if err := c.Do(ctx, "POST", fmt.Sprintf("/pods/%s/stop", url.PathEscape(id)), nil, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}
