package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const vpnConfigTemplateRevision = "20260612.8"

type vpnConfigArtifact struct {
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
	Content  string `json:"-"`
}

type vpnConfigManifest struct {
	UserID           int64                `json:"user_id"`
	UserName         string               `json:"user_name"`
	UUID             string               `json:"uuid"`
	UserUpdatedAt    string               `json:"user_updated_at"`
	TemplateRevision string               `json:"template_revision"`
	RuntimeChecksum  string               `json:"runtime_checksum"`
	Items            []vpnConfigArtifact  `json:"items"`
}

var vpnConfigKindLabels = map[string]string{
	"vless":         "VLESS 链接",
	"clash":         "Clash YAML（电脑）",
	"clash-android": "Clash YAML（Android）",
	"sing-box":      "sing-box JSON",
	"rocket":        "clg 小火箭 JSON",
}

func vpnConfigChecksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:12])
}

func vpnRuntimeChecksum(rt vpnRuntime) string {
	seed := strings.Join([]string{
		rt.ServerHost,
		fmt.Sprintf("%d", rt.Port),
		rt.SNI,
		rt.PublicKey,
		rt.ShortID,
	}, "|")
	return vpnConfigChecksum(seed)
}

func vpnConfigVersion(user vpnUser, rt vpnRuntime, kind string) string {
	userPart := "unknown"
	if !user.UpdatedAt.IsZero() {
		userPart = user.UpdatedAt.UTC().Format("20060102T150405")
	}
	return fmt.Sprintf("%s-%s-%s-%s", vpnConfigTemplateRevision, kind, userPart, vpnRuntimeChecksum(rt)[:8])
}

func embedYAMLMeta(body, version, checksum string) string {
	return fmt.Sprintf("# vpn-config-version: %s\n# vpn-config-checksum: %s\n", version, checksum) + body
}

func embedTextMeta(body, version, checksum string) string {
	return fmt.Sprintf("# vpn-config-version: %s\n# vpn-config-checksum: %s\n", version, checksum) + body
}

func newConfigArtifact(user vpnUser, rt vpnRuntime, kind, body string, embed func(string, string, string) string) vpnConfigArtifact {
	checksum := vpnConfigChecksum(body)
	version := vpnConfigVersion(user, rt, kind)
	content := body
	if embed != nil {
		content = embed(body, version, checksum)
	}
	label := vpnConfigKindLabels[kind]
	if label == "" {
		label = kind
	}
	return vpnConfigArtifact{
		Kind:     kind,
		Label:    label,
		Version:  version,
		Checksum: checksum,
		Content:  content,
	}
}

func buildVPNConfigManifest(user vpnUser, rt vpnRuntime) vpnConfigManifest {
	rocketArtifact, err := rocketConfigArtifact(user, rt)
	items := []vpnConfigArtifact{
		newConfigArtifact(user, rt, "vless", vlessURL(user, rt), embedTextMeta),
		newConfigArtifact(user, rt, "clash", clashConfig(user, rt), embedYAMLMeta),
		newConfigArtifact(user, rt, "clash-android", clashAndroidConfig(user, rt), embedYAMLMeta),
		newConfigArtifact(user, rt, "sing-box", singBoxClientConfig(user, rt), nil),
	}
	if err == nil {
		items = append(items, rocketArtifact)
	}
	userUpdatedAt := ""
	if !user.UpdatedAt.IsZero() {
		userUpdatedAt = user.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return vpnConfigManifest{
		UserID:           user.ID,
		UserName:         user.Name,
		UUID:             user.UUID,
		UserUpdatedAt:    userUpdatedAt,
		TemplateRevision: vpnConfigTemplateRevision,
		RuntimeChecksum:  vpnRuntimeChecksum(rt),
		Items:            items,
	}
}

func jsonConfigArtifact(user vpnUser, rt vpnRuntime, kind string, bodyBytes []byte) (vpnConfigArtifact, error) {
	artifact := newConfigArtifact(user, rt, kind, string(bodyBytes), nil)
	var enriched map[string]any
	if err := json.Unmarshal(bodyBytes, &enriched); err != nil {
		return vpnConfigArtifact{}, err
	}
	enriched["config_version"] = artifact.Version
	enriched["config_checksum"] = artifact.Checksum
	finalBytes, err := json.MarshalIndent(enriched, "", "  ")
	if err != nil {
		return vpnConfigArtifact{}, err
	}
	artifact.Content = string(finalBytes) + "\n"
	return artifact, nil
}

func rocketConfigArtifact(user vpnUser, rt vpnRuntime) (vpnConfigArtifact, error) {
	profile, err := rocketProfile(user, rt)
	if err != nil {
		return vpnConfigArtifact{}, err
	}
	bodyBytes, err := json.Marshal(profile)
	if err != nil {
		return vpnConfigArtifact{}, err
	}
	return jsonConfigArtifact(user, rt, "rocket", bodyBytes)
}

func manifestItem(m vpnConfigManifest, kind string) (vpnConfigArtifact, bool) {
	for _, item := range m.Items {
		if item.Kind == kind {
			return item, true
		}
	}
	return vpnConfigArtifact{}, false
}

func writeConfigArtifact(w http.ResponseWriter, artifact vpnConfigArtifact) {
	w.Header().Set("X-VPN-Config-Version", artifact.Version)
	w.Header().Set("X-VPN-Config-Checksum", artifact.Checksum)
	w.Header().Set("X-VPN-Config-Template-Revision", vpnConfigTemplateRevision)
	fmt.Fprint(w, artifact.Content)
}

func serveConfigArtifact(w http.ResponseWriter, user vpnUser, rt vpnRuntime, kind, filename, contentType string) {
	manifest := buildVPNConfigManifest(user, rt)
	artifact, ok := manifestItem(manifest, kind)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("config artifact %q not found", kind))
		return
	}
	w.Header().Set("Content-Type", contentType)
	if filename != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%s`, filename))
	}
	writeConfigArtifact(w, artifact)
}
