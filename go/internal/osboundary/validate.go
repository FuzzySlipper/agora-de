package osboundary

func ValidateDecisionRequest(request EscalationDecisionRequest) error {
	if request.EscalationID == "" {
		return ErrInvalidDecision
	}
	if request.DeciderUID == 0 {
		return ErrInvalidDecision
	}
	switch request.Decision {
	case EscalationDecisionApprove, EscalationDecisionDeny:
		return nil
	default:
		return ErrInvalidDecision
	}
}

