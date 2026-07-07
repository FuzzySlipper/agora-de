package theme

type TokenRole string

const (
	RolePresentation        TokenRole = "presentation"
	RoleLayout              TokenRole = "layout"
	RoleTypography          TokenRole = "typography"
	RoleEvidence            TokenRole = "evidence"
	RoleState               TokenRole = "state"
	TokenBackground                   = "--agora-bg"
	TokenForeground                   = "--agora-fg"
	TokenSurface                      = "--agora-surface"
	TokenSurfaceRaised                = "--agora-surface-raised"
	TokenSurfaceStrong                = "--agora-surface-strong"
	TokenTextMuted                    = "--agora-text-muted"
	TokenAccent                       = "--agora-accent"
	TokenWarning                      = "--agora-warning"
	TokenBorder                       = "--agora-border"
	TokenBorderSubtle                 = "--agora-border-subtle"
	TokenFontFamily                   = "--agora-font-family"
	TokenFontBackground               = "--agora-font-background"
	TokenFontPanel                    = "--agora-font-panel"
	TokenFontStatus                   = "--agora-font-status"
	TokenFontCode                     = "--agora-font-code"
	TokenRadiusControl                = "--agora-radius-control"
	TokenPanelHeight                  = "--agora-panel-height"
	TokenPanelGap                     = "--agora-panel-gap"
	TokenPanelPaddingX                = "--agora-panel-padding-x"
	TokenControlHeight                = "--agora-control-height"
	TokenEvidenceBackground           = "--agora-evidence-bg"
	TokenEvidenceAccent               = "--agora-evidence-accent"
	TokenEvidenceStrong               = "--agora-evidence-strong"
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
		{Name: TokenRadiusControl, Value: "4px", Role: RoleLayout, Description: "Shared radius for controls, badges, and app chips."},
		{Name: TokenPanelHeight, Value: "96px", Role: RoleLayout, Description: "Installed panel height."},
		{Name: TokenPanelGap, Value: "14px", Role: RoleLayout, Description: "Panel item gap."},
		{Name: TokenPanelPaddingX, Value: "22px", Role: RoleLayout, Description: "Horizontal panel padding."},
		{Name: TokenControlHeight, Value: "44px", Role: RoleLayout, Description: "Panel button and compact control height."},
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
