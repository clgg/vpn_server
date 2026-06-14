package main

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const singBoxAccessLogPath = "/var/lib/sing-box/access.log"

var (
	singBoxConnFromRE = regexp.MustCompile(`\[(\d+) \d+ms\] inbound/vless\[vless-reality-in\]: inbound connection from ([0-9a-fA-F.:]+):`)
	singBoxConnUserRE = regexp.MustCompile(`\[(\d+) \d+ms\] inbound/vless\[vless-reality-in\]: \[([^\]]+)\] inbound connection to `)
	xrayAcceptedRE    = regexp.MustCompile(`from ([0-9a-fA-F.:]+):\d+ accepted .* email: (\S+)`)
)

type userIPBinding struct {
	userName string
	lastSeen time.Time
}

type userIPTracker struct {
	mu   sync.RWMutex
	byIP map[string]userIPBinding
}

var sessionTracker userIPTracker

func init() {
	sessionTracker.byIP = make(map[string]userIPBinding)
}

func (t *userIPTracker) remember(sourceIP, userName string, seenAt time.Time) {
	sourceIP = strings.TrimSpace(sourceIP)
	userName = strings.TrimSpace(userName)
	if sourceIP == "" || userName == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.byIP[sourceIP] = userIPBinding{userName: userName, lastSeen: seenAt}
}

func (t *userIPTracker) lookup(sourceIP string) string {
	sourceIP = strings.TrimSpace(sourceIP)
	if sourceIP == "" {
		return ""
	}
	now := time.Now().UTC()
	cutoff := now.Add(-15 * time.Minute)
	t.mu.Lock()
	defer t.mu.Unlock()
	binding, ok := t.byIP[sourceIP]
	if !ok {
		return ""
	}
	if binding.lastSeen.Before(cutoff) {
		delete(t.byIP, sourceIP)
		return ""
	}
	return binding.userName
}

func refreshUserIPMappings(ctx context.Context) {
	if output, err := xrayAccessLogTail(ctx); err == nil {
		parseXrayAccessLog(output)
		return
	}
	output, err := singBoxAccessLogTail(ctx)
	if err != nil {
		log.Printf("refresh user ip mappings: %v", err)
		return
	}
	parseSingBoxAccessLog(output)
}

func parseXrayAccessLog(output []byte) {
	now := time.Now().UTC()
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := xrayAcceptedRE.FindStringSubmatch(line)
		if len(matches) != 3 {
			continue
		}
		sessionTracker.remember(matches[1], matches[2], now)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("parse xray access log: %v", err)
	}
}

func xrayAccessLogTail(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "tail", "-n", "4000", xrayAccessLogPath)
	return cmd.Output()
}

func parseSingBoxAccessLog(output []byte) {
	now := time.Now().UTC()
	connIPs := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := singBoxConnFromRE.FindStringSubmatch(line); len(matches) == 3 {
			connIPs[matches[1]] = matches[2]
			continue
		}
		if matches := singBoxConnUserRE.FindStringSubmatch(line); len(matches) == 3 {
			sourceIP := connIPs[matches[1]]
			if sourceIP == "" {
				continue
			}
			sessionTracker.remember(sourceIP, matches[2], now)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("parse sing-box access log: %v", err)
	}
}

func singBoxAccessLogTail(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "tail", "-n", "4000", singBoxAccessLogPath)
	return cmd.Output()
}

func userNameForSourceIP(sourceIP string) string {
	return sessionTracker.lookup(sourceIP)
}

func isVPNInboundConnection(meta clashConnectionMetadata) bool {
	if tag := strings.TrimSpace(meta.InboundName); tag != "" && tag != vpnInboundTag {
		return false
	}
	connType := strings.TrimSpace(meta.Type)
	if connType != "" {
		return strings.HasSuffix(connType, "/"+vpnInboundTag) || strings.Contains(connType, vpnInboundTag)
	}
	return true
}

func clashConnectionUser(meta clashConnectionMetadata, sourceIP string) string {
	for _, value := range []string{meta.InboundUser, meta.User} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return userNameForSourceIP(sourceIP)
}
