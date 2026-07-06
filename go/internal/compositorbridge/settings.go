package compositorbridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	LayoutRuleZones       = "zones"
	LayoutRuleMasterStack = "master_stack"
	LayoutRuleDwindle     = "dwindle"
)

func DefaultLayoutSettings() LayoutSettings {
	return LayoutSettings{
		Rule:        LayoutRuleMasterStack,
		Mode:        LayoutModeZones,
		Gaps:        LayoutGaps{},
		MasterCount: 1,
		MasterRatio: 0.5,
		SmartGaps:   true,
	}
}

func LoadLayoutSettings(path string) (LayoutSettings, error) {
	settings := DefaultLayoutSettings()
	if path == "" {
		return settings, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return LayoutSettings{}, fmt.Errorf("read layout settings: %w", err)
	}
	if err := json.Unmarshal(payload, &settings); err != nil {
		return LayoutSettings{}, fmt.Errorf("decode layout settings: %w", err)
	}
	if err := validateLayoutSettings(settings); err != nil {
		return LayoutSettings{}, err
	}
	return settings, nil
}

func SaveLayoutSettings(path string, settings LayoutSettings) error {
	if path == "" {
		return nil
	}
	if err := validateLayoutSettings(settings); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir layout settings dir: %w", err)
	}
	payload, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode layout settings: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("write layout settings: %w", err)
	}
	return nil
}

func validateLayoutSettings(settings LayoutSettings) error {
	switch settings.Rule {
	case LayoutRuleZones, LayoutRuleMasterStack, LayoutRuleDwindle:
	default:
		return fmt.Errorf("unsupported layout rule %q", settings.Rule)
	}
	if !validLayoutMode(settings.Mode) {
		return fmt.Errorf("unsupported layout mode %q", settings.Mode)
	}
	if settings.MasterCount <= 0 {
		return fmt.Errorf("master_count must be positive")
	}
	if settings.MasterRatio < 0.1 || settings.MasterRatio > 0.9 {
		return fmt.Errorf("master_ratio must be between 0.1 and 0.9")
	}
	for name, value := range map[string]int{
		"outer_horizontal": settings.Gaps.OuterHorizontal,
		"outer_vertical":   settings.Gaps.OuterVertical,
		"inner_horizontal": settings.Gaps.InnerHorizontal,
		"inner_vertical":   settings.Gaps.InnerVertical,
	} {
		if value < 0 {
			return fmt.Errorf("%s gap must be non-negative", name)
		}
	}
	return nil
}
