package gatewayadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DeviceEventEnvelope struct {
	ProtocolVersion   string        `json:"protocol_version"`
	SourceType        string        `json:"source_type"`
	MessageID         string        `json:"message_id"`
	GatewayName       string        `json:"gateway_name"`
	GatewayExternalID string        `json:"gateway_external_id"`
	RemoteIP          string        `json:"remote_ip"`
	RemotePort        int           `json:"remote_port"`
	ReportedAtMS      string        `json:"reported_at_ms"`
	Events            []DeviceEvent `json:"events"`
}

type DeviceEvent struct {
	EventType        string   `json:"event_type"`
	Frequency        int      `json:"frequency,omitempty"`
	DeviceExternalID string   `json:"device_external_id,omitempty"`
	FunctionType     uint8    `json:"function_type,omitempty"`
	ChannelNo        int      `json:"channel_no,omitempty"`
	Value            *float64 `json:"value,omitempty"`
	Unit             string   `json:"unit,omitempty"`
}

type RawFrameEnvelope struct {
	ProtocolVersion string `json:"protocol_version"`
	SourceType      string `json:"source_type"`
	MessageID       string `json:"message_id"`
	ConnectionID    string `json:"connection_id"`
	GatewayName     string `json:"gateway_name,omitempty"`
	RemoteIP        string `json:"remote_ip,omitempty"`
	RemotePort      int    `json:"remote_port,omitempty"`
	Direction       string `json:"direction"`
	PayloadHex      string `json:"payload_hex"`
	ReceivedAt      string `json:"received_at"`
	RawSeq          uint64 `json:"raw_seq"`
}

type PendingCommand struct {
	CommandID  uint64 `json:"command_id,string"`
	CommandHex string `json:"command_hex"`
}

type Forwarder struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewForwarder(baseURL, token string) *Forwarder {
	return &Forwarder{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (f *Forwarder) Ready(ctx context.Context) error {
	if f.baseURL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("go service readyz returned %s", resp.Status)
	}
	return nil
}

func (f *Forwarder) PostDeviceEvents(ctx context.Context, envelope DeviceEventEnvelope) error {
	if f.baseURL == "" {
		return nil
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/api/v1/adapter/device-events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Adapter-Token", f.token)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("device-events returned %s", resp.Status)
	}
	return nil
}

func (f *Forwarder) PostRawFrame(ctx context.Context, envelope RawFrameEnvelope) error {
	if f.baseURL == "" {
		return nil
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/api/v1/adapter/raw-frames", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Adapter-Token", f.token)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("raw-frames returned %s", resp.Status)
	}
	return nil
}

func (f *Forwarder) ListPendingCommands(ctx context.Context, gatewayExternalID string, limit int) ([]PendingCommand, error) {
	if f.baseURL == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	query := url.Values{}
	query.Set("gateway_external_id", gatewayExternalID)
	query.Set("limit", strconv.Itoa(limit))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"/api/v1/adapter/commands/pending?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Adapter-Token", f.token)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("pending commands returned %s", resp.Status)
	}
	var payload struct {
		Data struct {
			Items []PendingCommand `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Data.Items, nil
}

func (f *Forwarder) MarkCommandSent(ctx context.Context, commandID uint64) error {
	if f.baseURL == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/api/v1/adapter/commands/"+strconv.FormatUint(commandID, 10)+"/sent", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Adapter-Token", f.token)
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mark command sent returned %s", resp.Status)
	}
	return nil
}
