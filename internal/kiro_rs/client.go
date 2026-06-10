package kiro_rs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultRegion     = "us-east-1"
	DefaultProfileARN = "arn:aws:codewhisperer:us-east-1:638616132270:profile/AAAACCCCXXXX"
)

type UploadPayload struct {
	RefreshToken  string `json:"refreshToken"`
	ProfileARN    string `json:"profileArn"`
	AuthMethod    string `json:"authMethod"`
	ClientID      string `json:"clientId"`
	ClientSecret  string `json:"clientSecret"`
	Region        string `json:"region"`
	AuthRegion    string `json:"authRegion"`
	APIRegion     string `json:"apiRegion"`
	MachineID     string `json:"machineId"`
	Email         string `json:"email"`
	ProxyURL      string `json:"proxyUrl,omitempty"`
	ProxyUsername string `json:"proxyUsername,omitempty"`
	ProxyPassword string `json:"proxyPassword,omitempty"`
}

type UploadResult struct {
	CredentialID int                    `json:"credentialId"`
	Email        string                 `json:"email"`
	Message      string                 `json:"message"`
	Raw          map[string]interface{} `json:"raw,omitempty"`
}

type ConnectionResult struct {
	OK      bool   `json:"ok"`
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func NormalizeBaseURL(value string) (string, error) {
	normalized := strings.TrimRight(strings.TrimSpace(value), "/")
	if normalized == "" {
		return "", fmt.Errorf("缺少 kiro.rs 管理后台地址")
	}
	if strings.HasSuffix(normalized, "/admin") {
		normalized = strings.TrimSuffix(normalized, "/admin")
	}
	return normalized, nil
}

func BuildMachineID(refreshToken string) (string, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return "", fmt.Errorf("缺少 refreshToken，无法生成 machineId")
	}
	sum := sha256.Sum256([]byte("KotlinNativeAPI/" + refreshToken))
	return hex.EncodeToString(sum[:]), nil
}

func BuildAdminHeaders(apiKey string) map[string]string {
	headers := map[string]string{"Accept": "application/json"}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" {
		headers["x-api-key"] = apiKey
		headers["Authorization"] = "Bearer " + apiKey
	}
	return headers
}

func BuildPayloadFromResult(result map[string]interface{}, proxyURL string) (UploadPayload, error) {
	var payload UploadPayload
	if result == nil {
		return payload, fmt.Errorf("注册结果为空")
	}
	email, _ := result["email"].(string)
	at, _ := result["aws_token"].(map[string]interface{})
	refreshToken, _ := at["refreshToken"].(string)
	clientID, _ := result["client_id"].(string)
	clientSecret, _ := result["client_secret"].(string)

	email = strings.TrimSpace(email)
	refreshToken = strings.TrimSpace(refreshToken)
	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)
	if email == "" {
		return payload, fmt.Errorf("缺少 email")
	}
	if refreshToken == "" {
		return payload, fmt.Errorf("缺少 refreshToken")
	}
	if clientID == "" {
		return payload, fmt.Errorf("缺少 clientId")
	}
	if clientSecret == "" {
		return payload, fmt.Errorf("缺少 clientSecret")
	}
	machineID, err := BuildMachineID(refreshToken)
	if err != nil {
		return payload, err
	}
	payload = UploadPayload{
		RefreshToken: refreshToken,
		ProfileARN:   DefaultProfileARN,
		AuthMethod:   "idc",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Region:       DefaultRegion,
		AuthRegion:   DefaultRegion,
		APIRegion:    DefaultRegion,
		MachineID:    machineID,
		Email:        email,
		ProxyURL:     strings.TrimSpace(proxyURL),
	}
	return payload, nil
}

func CheckConnection(ctx context.Context, baseURL, apiKey string) ConnectionResult {
	normalizedBaseURL, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return ConnectionResult{OK: false, Message: err.Error()}
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedBaseURL+"/api/admin/credentials", nil)
	if err != nil {
		return ConnectionResult{OK: false, Message: err.Error()}
	}
	for k, v := range BuildAdminHeaders(apiKey) {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ConnectionResult{OK: false, Message: err.Error()}
	}
	defer resp.Body.Close()
	body := readResponse(resp.Body)
	detail := readResponseMessage(body, resp.Status)
	switch resp.StatusCode {
	case http.StatusOK:
		return ConnectionResult{OK: true, Status: resp.StatusCode, Message: fmt.Sprintf("kiro.rs 连接正常（HTTP %d）", resp.StatusCode)}
	case http.StatusMethodNotAllowed:
		return ConnectionResult{OK: true, Status: resp.StatusCode, Message: "kiro.rs 上传接口可访问"}
	case http.StatusUnauthorized, http.StatusForbidden:
		return ConnectionResult{OK: false, Status: resp.StatusCode, Message: fmt.Sprintf("kiro.rs API Key 被拒绝（HTTP %d%s）", resp.StatusCode, suffixDetail(detail))}
	case http.StatusNotFound:
		return ConnectionResult{OK: false, Status: resp.StatusCode, Message: fmt.Sprintf("未找到 kiro.rs 管理接口（HTTP 404%s）", suffixDetail(detail))}
	default:
		if detail == "" {
			detail = fmt.Sprintf("kiro.rs 连接失败（HTTP %d）", resp.StatusCode)
		}
		return ConnectionResult{OK: false, Status: resp.StatusCode, Message: detail}
	}
}

func UploadCredential(ctx context.Context, baseURL, apiKey string, payload UploadPayload) (UploadResult, error) {
	var result UploadResult
	normalizedBaseURL, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return result, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizedBaseURL+"/api/admin/credentials", bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	for k, v := range BuildAdminHeaders(apiKey) {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	respBody := readResponse(resp.Body)
	if !statusOK(resp.StatusCode) {
		message := readResponseMessage(respBody, resp.Status)
		if message == "" {
			message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return result, fmt.Errorf("kiro.rs 凭据上传失败: %s", message)
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(respBody, &raw)
	result.Raw = raw
	result.CredentialID = int(numberField(raw, "credentialId", "credential_id"))
	result.Email, _ = raw["email"].(string)
	msg, _ := raw["message"].(string)
	result.Message = NormalizeUploadMessage(msg)
	return result, nil
}

func statusOK(code int) bool { return code >= 200 && code < 300 }

func NormalizeUploadMessage(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "上传成功"
	}
	lower := strings.ToLower(raw)
	if lower == "uploaded" || lower == "credential uploaded." {
		return "上传成功"
	}
	return raw
}

func readResponse(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

func readResponseMessage(body []byte, fallback string) string {
	var obj map[string]interface{}
	if len(body) > 0 && json.Unmarshal(body, &obj) == nil {
		if errObj, _ := obj["error"].(map[string]interface{}); errObj != nil {
			if msg, _ := errObj["message"].(string); strings.TrimSpace(msg) != "" {
				return strings.TrimSpace(msg)
			}
		}
		if msg, _ := obj["message"].(string); strings.TrimSpace(msg) != "" {
			return strings.TrimSpace(msg)
		}
	}
	text := strings.TrimSpace(string(body))
	if text != "" {
		return text
	}
	return strings.TrimSpace(fallback)
}

func suffixDetail(detail string) string {
	if strings.TrimSpace(detail) == "" {
		return ""
	}
	return "：" + strings.TrimSpace(detail)
}

func numberField(m map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return 0
}
