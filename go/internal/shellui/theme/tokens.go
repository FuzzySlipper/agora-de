package theme

type TokenRole string

const (
	RolePresentation                TokenRole = "presentation"
	RoleLayout                      TokenRole = "layout"
	RoleTypography                  TokenRole = "typography"
	RoleEvidence                    TokenRole = "evidence"
	RoleState                       TokenRole = "state"
	RoleComponent                   TokenRole = "component"
	TokenBackground                           = "--agora-bg"
	TokenForeground                           = "--agora-fg"
	TokenSurface                              = "--agora-surface"
	TokenSurfaceRaised                        = "--agora-surface-raised"
	TokenSurfaceStrong                        = "--agora-surface-strong"
	TokenTextMuted                            = "--agora-text-muted"
	TokenAccent                               = "--agora-accent"
	TokenWarning                              = "--agora-warning"
	TokenBorder                               = "--agora-border"
	TokenBorderSubtle                         = "--agora-border-subtle"
	TokenFontFamily                           = "--agora-font-family"
	TokenFontBackground                       = "--agora-font-background"
	TokenFontPanel                            = "--agora-font-panel"
	TokenFontStatus                           = "--agora-font-status"
	TokenFontCode                             = "--agora-font-code"
	TokenFontHeading                          = "--agora-font-heading"
	TokenFontTitle                            = "--agora-font-title"
	TokenFontBody                             = "--agora-font-body"
	TokenFontCaption                          = "--agora-font-caption"
	TokenLineHeightTight                      = "--agora-line-height-tight"
	TokenLineHeightNormal                     = "--agora-line-height-normal"
	TokenRadiusControl                        = "--agora-radius-control"
	TokenPanelHeight                          = "--agora-panel-height"
	TokenPanelGap                             = "--agora-panel-gap"
	TokenPanelPaddingX                        = "--agora-panel-padding-x"
	TokenControlHeight                        = "--agora-control-height"
	TokenSpace1                               = "--agora-space-1"
	TokenSpace2                               = "--agora-space-2"
	TokenSpace3                               = "--agora-space-3"
	TokenSpace4                               = "--agora-space-4"
	TokenSpace5                               = "--agora-space-5"
	TokenPanelControlHeight                   = "--agora-panel-control-height"
	TokenPanelBackground                      = "--agora-panel-bg"
	TokenPanelShadow                          = "--agora-panel-shadow"
	TokenPopupShadow                          = "--agora-popup-shadow"
	TokenOverlayLabelBackground               = "--agora-overlay-label-bg"
	TokenOverlayChipBackground                = "--agora-overlay-chip-bg"
	TokenFocusGlow                            = "--agora-focus-glow"
	TokenTaskbarMinimizedBackground           = "--agora-taskbar-minimized-bg"
	TokenTaskbarMinimizedBorder               = "--agora-taskbar-minimized-border"
	TokenDuration                             = "--agora-duration"
	TokenEase                                 = "--agora-ease"
	TokenElevation1                           = "--agora-elevation-1"
	TokenElevation2                           = "--agora-elevation-2"
	TokenElevation3                           = "--agora-elevation-3"
	TokenEvidenceBackground                   = "--agora-evidence-bg"
	TokenEvidenceAccent                       = "--agora-evidence-accent"
	TokenEvidenceStrong                       = "--agora-evidence-strong"
)

type TokenDefinition struct {
	Name        string    `json:"name"`
	Value       string    `json:"value"`
	Role        TokenRole `json:"role"`
	Description string    `json:"description"`
}

func DefaultTokenDefinitions() []TokenDefinition {
	return []TokenDefinition{
		{Name: TokenBackground, Value: "#0f172a", Role: RolePresentation, Description: "Canvas background for shell fallback and chrome surfaces."},
		{Name: TokenForeground, Value: "#e5eef7", Role: RolePresentation, Description: "Primary text color for shell fallback and chrome surfaces."},
		{Name: TokenSurface, Value: "#111827", Role: RolePresentation, Description: "Panel and plain section background."},
		{Name: TokenSurfaceRaised, Value: "#1f2937", Role: RolePresentation, Description: "Raised controls, list items, and code blocks."},
		{Name: TokenSurfaceStrong, Value: "#020617", Role: RolePresentation, Description: "High-contrast badges and icon chips."},
		{Name: TokenTextMuted, Value: "#94a3b8", Role: RolePresentation, Description: "Secondary labels and muted details."},
		{Name: TokenAccent, Value: "#5eead4", Role: RolePresentation, Description: "Interactive accent for normal shell presentation."},
		{Name: TokenWarning, Value: "#fbbf24", Role: RoleState, Description: "Warning border and status color."},
		{Name: TokenBorder, Value: "#64748b", Role: RolePresentation, Description: "Default control border."},
		{Name: TokenBorderSubtle, Value: "#334155", Role: RolePresentation, Description: "Subtle dividers and inactive item borders."},
		{Name: TokenFontFamily, Value: "system-ui, sans-serif", Role: RoleTypography, Description: "Base UI font family."},
		{Name: TokenFontBackground, Value: "600 22px var(--agora-font-family)", Role: RoleTypography, Description: "Background shell label font."},
		{Name: TokenFontPanel, Value: "600 18px var(--agora-font-family)", Role: RoleTypography, Description: "Panel control font."},
		{Name: TokenFontStatus, Value: "600 16px var(--agora-font-family)", Role: RoleTypography, Description: "Operator/status surface font."},
		{Name: TokenFontCode, Value: "600 13px ui-monospace, SFMono-Regular, Menlo, monospace", Role: RoleTypography, Description: "Code and recovery command font."},
		{Name: TokenFontHeading, Value: "700 24px var(--agora-font-family)", Role: RoleTypography, Description: "Heading font for shell titles and section headers."},
		{Name: TokenFontTitle, Value: "700 18px var(--agora-font-family)", Role: RoleTypography, Description: "Title font for cards and surface headers."},
		{Name: TokenFontBody, Value: "400 15px var(--agora-font-family)", Role: RoleTypography, Description: "Body text font for list items and descriptions."},
		{Name: TokenFontCaption, Value: "500 12px var(--agora-font-family)", Role: RoleTypography, Description: "Caption font for metadata and secondary labels."},
		{Name: TokenLineHeightTight, Value: "1.2", Role: RoleTypography, Description: "Tight line height for headings and single-line labels."},
		{Name: TokenLineHeightNormal, Value: "1.5", Role: RoleTypography, Description: "Normal line height for body and multi-line text."},
		{Name: TokenRadiusControl, Value: "4px", Role: RoleLayout, Description: "Shared radius for controls, badges, and app chips."},
		{Name: TokenPanelHeight, Value: "96px", Role: RoleLayout, Description: "Installed panel height."},
		{Name: TokenPanelGap, Value: "14px", Role: RoleLayout, Description: "Panel item gap."},
		{Name: TokenPanelPaddingX, Value: "22px", Role: RoleLayout, Description: "Horizontal panel padding."},
		{Name: TokenControlHeight, Value: "44px", Role: RoleLayout, Description: "Panel button and compact control height."},
		{Name: TokenSpace1, Value: "4px", Role: RoleLayout, Description: "Smallest spacing step for tight gaps and icon insets."},
		{Name: TokenSpace2, Value: "6px", Role: RoleLayout, Description: "Small spacing step for control clusters."},
		{Name: TokenSpace3, Value: "8px", Role: RoleLayout, Description: "Base spacing step for item gaps and padding."},
		{Name: TokenSpace4, Value: "10px", Role: RoleLayout, Description: "Comfortable spacing step for control padding."},
		{Name: TokenSpace5, Value: "14px", Role: RoleLayout, Description: "Large spacing step for section gaps."},
		{Name: TokenPanelControlHeight, Value: "38px", Role: RoleComponent, Description: "Taskbar panel control height for dense shell controls."},
		{Name: TokenPanelBackground, Value: "color-mix(in srgb, var(--agora-surface) 92%, var(--agora-bg))", Role: RoleComponent, Description: "Taskbar panel background treatment."},
		{Name: TokenPanelShadow, Value: "inset 0 1px 0 var(--agora-border-subtle), 0 -10px 26px rgba(0, 0, 0, 0.28)", Role: RoleComponent, Description: "Taskbar panel top inset and lift shadow."},
		{Name: TokenPopupShadow, Value: "0 18px 60px rgba(0, 0, 0, 0.42)", Role: RoleComponent, Description: "Launcher, status, and WM popup shadow."},
		{Name: TokenOverlayLabelBackground, Value: "rgba(8, 13, 30, 0.72)", Role: RoleComponent, Description: "Agent overlay label background."},
		{Name: TokenOverlayChipBackground, Value: "rgba(8, 13, 30, 0.62)", Role: RoleComponent, Description: "Agent overlay compact chip background."},
		{Name: TokenFocusGlow, Value: "0 0 22px rgba(251, 191, 36, 0.75)", Role: RoleComponent, Description: "Focused overlay surface glow."},
		{Name: TokenTaskbarMinimizedBackground, Value: "color-mix(in srgb, var(--agora-surface-raised) 74%, var(--agora-bg))", Role: RoleComponent, Description: "Taskbar button background for minimized surfaces."},
		{Name: TokenTaskbarMinimizedBorder, Value: "color-mix(in srgb, var(--agora-warning) 74%, var(--agora-border))", Role: RoleComponent, Description: "Taskbar button border for minimized surfaces."},
		{Name: TokenDuration, Value: "120ms", Role: RoleComponent, Description: "Default transition duration for hover and focus states."},
		{Name: TokenEase, Value: "cubic-bezier(0.2, 0.8, 0.2, 1)", Role: RoleComponent, Description: "Default easing curve for shell motion."},
		{Name: TokenElevation1, Value: "0 1px 2px rgba(0, 0, 0, 0.20)", Role: RoleComponent, Description: "Resting elevation for cards and chips."},
		{Name: TokenElevation2, Value: "0 6px 18px rgba(0, 0, 0, 0.28)", Role: RoleComponent, Description: "Raised elevation for panels and bars."},
		{Name: TokenElevation3, Value: "0 18px 60px rgba(0, 0, 0, 0.42)", Role: RoleComponent, Description: "Top elevation for popups and overlays."},
		{Name: TokenEvidenceBackground, Value: "#0f172a", Role: RoleEvidence, Description: "Stable visible-shell evidence background marker; not a general customization token."},
		{Name: TokenEvidenceAccent, Value: "#00d1b2", Role: RoleEvidence, Description: "Stable visible-shell evidence accent marker used by capture classifiers."},
		{Name: TokenEvidenceStrong, Value: "#020617", Role: RoleEvidence, Description: "Stable visible-shell evidence strong marker used by capture classifiers."},
	}
}

func DefaultTokens() map[string]string {
	definitions := DefaultTokenDefinitions()
	tokens := make(map[string]string, len(definitions))
	for _, definition := range definitions {
		tokens[definition.Name] = definition.Value
	}
	return tokens
}

func DefaultManifest() Manifest {
	return Manifest{
		ID:     DefaultThemeID,
		Name:   "Agora Observatory",
		Tokens: DefaultTokens(),
	}
}

func DefaultTokenCSS() (string, error) {
	return SafeTokenCSS(DefaultTokens())
}

func MustDefaultTokenCSS() string {
	css, err := DefaultTokenCSS()
	if err != nil {
		panic(err)
	}
	return css
}
