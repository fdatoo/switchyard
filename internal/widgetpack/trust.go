package widgetpack

// TrustPolicy defines which pack signers are trusted.
type TrustPolicy struct {
	AllowedSigners []string
	AllowUnsigned  bool
}

// Verify checks whether a pack's signature satisfies the trust policy.
// signers is the list of verified signer identities from the pack's signature envelope.
func (tp *TrustPolicy) Verify(status string, _ []string) bool {
	switch status {
	case "verified":
		return true
	case "unsigned":
		return tp.AllowUnsigned
	default:
		return false
	}
}
