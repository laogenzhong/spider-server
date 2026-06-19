package router

import (
	"context"
	pb "spider-server/gen/spider/api"
	mysqlmodel "spider-server/mysql/model"
	"strconv"
	"strings"
)

type AppUpdateApi struct {
	pb.UnimplementedAppUpdateServiceServer
}

func (a *AppUpdateApi) CheckAppUpdate(ctx context.Context, req *pb.AppUpdateCheckRequest) (*pb.AppUpdateCheckResponse, error) {
	updateConfig, err := mysqlmodel.GetAppUpdateConfig(mysqlmodel.AppUpdatePlatformIOS)
	if err != nil {
		return &pb.AppUpdateCheckResponse{}, nil
	}

	resp := &pb.AppUpdateCheckResponse{
		LatestVersion:       strings.TrimSpace(updateConfig.LatestVersion),
		MinSupportedVersion: strings.TrimSpace(updateConfig.MinSupportedVersion),
		AppStoreUrl:         strings.TrimSpace(updateConfig.AppStoreURL),
		Message:             updateConfig.MessageForLanguage(req.GetSystemLanguage()),
	}

	if strings.EqualFold(strings.TrimSpace(req.GetPlatform()), "ios") || strings.TrimSpace(req.GetPlatform()) == "" {
		currentVersion := strings.TrimSpace(req.GetCurrentVersion())
		resp.UpdateAvailable = updateConfig.UpdateAvailableEnabled && compareAppVersions(currentVersion, updateConfig.LatestVersion) < 0
		resp.ForceUpdate = updateConfig.ForceUpdateEnabled && compareAppVersions(currentVersion, updateConfig.MinSupportedVersion) < 0
	}

	return resp, nil
}

func compareAppVersions(current string, target string) int {
	current = strings.TrimSpace(current)
	target = strings.TrimSpace(target)
	if current == "" || target == "" {
		return 0
	}

	currentParts := splitAppVersion(current)
	targetParts := splitAppVersion(target)
	maxLen := len(currentParts)
	if len(targetParts) > maxLen {
		maxLen = len(targetParts)
	}

	for i := 0; i < maxLen; i++ {
		currentPart := 0
		if i < len(currentParts) {
			currentPart = currentParts[i]
		}
		targetPart := 0
		if i < len(targetParts) {
			targetPart = targetParts[i]
		}
		if currentPart < targetPart {
			return -1
		}
		if currentPart > targetPart {
			return 1
		}
	}
	return 0
}

func splitAppVersion(version string) []int {
	rawParts := strings.FieldsFunc(version, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == '+'
	})
	parts := make([]int, 0, len(rawParts))
	for _, raw := range rawParts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			parts = append(parts, 0)
			continue
		}
		numericEnd := 0
		for numericEnd < len(raw) && raw[numericEnd] >= '0' && raw[numericEnd] <= '9' {
			numericEnd++
		}
		if numericEnd == 0 {
			parts = append(parts, 0)
			continue
		}
		value, err := strconv.Atoi(raw[:numericEnd])
		if err != nil {
			value = 0
		}
		parts = append(parts, value)
	}
	return parts
}
