// Package audit emits AuthEvent payloads to the event store. Every auth and
// policy decision that surfaces in the audit log goes through one of the
// emit-helpers below.
package audit

import (
	"context"

	eventv1 "github.com/fdatoo/switchyard/gen/switchyard/event/v1"
	"github.com/fdatoo/switchyard/internal/eventstore"
)

type Recorder struct {
	es eventstore.Appender
}

func New(es eventstore.Appender) *Recorder { return &Recorder{es: es} }

type Identity struct {
	PrincipalID string
	SourceIP    string
	UserAgent   string
	RequestID   string
}

func (i Identity) toProto() *eventv1.Identity {
	return &eventv1.Identity{
		PrincipalId: i.PrincipalID, SourceIp: i.SourceIP,
		UserAgent: i.UserAgent, RequestId: i.RequestID,
	}
}

func (r *Recorder) emit(ctx context.Context, id Identity, kind interface{}) error {
	e := &eventv1.AuthEvent{Identity: id.toProto()}
	switch k := kind.(type) {
	case LoginSucceeded:
		e.Kind = &eventv1.AuthEvent_LoginSucceeded{LoginSucceeded: &eventv1.LoginSucceeded{
			AuthMethod: k.AuthMethod, UserSlug: k.UserSlug,
			SessionId: k.SessionID, CredentialId: k.CredentialID,
		}}
	case LoginFailed:
		e.Kind = &eventv1.AuthEvent_LoginFailed{LoginFailed: &eventv1.LoginFailed{
			AuthMethod: k.AuthMethod, AttemptedUserSlug: k.AttemptedUserSlug, Reason: k.Reason,
		}}
	case Logout:
		e.Kind = &eventv1.AuthEvent_Logout{Logout: &eventv1.Logout{UserSlug: k.UserSlug, SessionId: k.SessionID}}
	case SessionRefreshed:
		e.Kind = &eventv1.AuthEvent_SessionRefreshed{SessionRefreshed: &eventv1.SessionRefreshed{
			UserSlug: k.UserSlug, SessionId: k.SessionID, NewSessionId: k.NewSessionID,
		}}
	case SessionReplayDetected:
		e.Kind = &eventv1.AuthEvent_SessionReplayDetected{SessionReplayDetected: &eventv1.SessionReplayDetected{
			UserSlug: k.UserSlug, SessionId: k.SessionID, RevokedSessionCount: k.RevokedCount,
		}}
	case PasswordChanged:
		e.Kind = &eventv1.AuthEvent_PasswordChanged{PasswordChanged: &eventv1.PasswordChanged{UserSlug: k.UserSlug, SetBy: k.SetBy}}
	case PasskeyRegistered:
		e.Kind = &eventv1.AuthEvent_PasskeyRegistered{PasskeyRegistered: &eventv1.PasskeyRegistered{
			UserSlug: k.UserSlug, CredentialId: k.CredentialID, Label: k.Label,
		}}
	case PasskeyUnregistered:
		e.Kind = &eventv1.AuthEvent_PasskeyUnregistered{PasskeyUnregistered: &eventv1.PasskeyUnregistered{
			UserSlug: k.UserSlug, CredentialId: k.CredentialID, Label: k.Label,
		}}
	case EnrollmentTokenMinted:
		e.Kind = &eventv1.AuthEvent_EnrollmentTokenMinted{EnrollmentTokenMinted: &eventv1.EnrollmentTokenMinted{
			UserSlug: k.UserSlug, Intent: k.Intent, ExpiresAt: k.ExpiresAt,
		}}
	case EnrollmentTokenRedeemed:
		e.Kind = &eventv1.AuthEvent_EnrollmentTokenRedeemed{EnrollmentTokenRedeemed: &eventv1.EnrollmentTokenRedeemed{
			UserSlug: k.UserSlug, Intent: k.Intent,
		}}
	case TokenMinted:
		e.Kind = &eventv1.AuthEvent_TokenMinted{TokenMinted: &eventv1.TokenMinted{
			UserSlug: k.UserSlug, TokenId: k.TokenID, Label: k.Label,
			ScopeSummary: k.ScopeSummary, TtlSeconds: k.TTLSeconds, IssuedByPrincipalId: k.IssuedBy,
		}}
	case TokenRevoked:
		e.Kind = &eventv1.AuthEvent_TokenRevoked{TokenRevoked: &eventv1.TokenRevoked{
			TokenId: k.TokenID, RevokedByPrincipalId: k.RevokedBy, Reason: k.Reason,
		}}
	case TokenRejected:
		e.Kind = &eventv1.AuthEvent_TokenRejected{TokenRejected: &eventv1.TokenRejected{
			TokenIdPrefix: k.TokenIDPrefix, Reason: k.Reason,
		}}
	case PolicyDenied:
		e.Kind = &eventv1.AuthEvent_PolicyDenied{PolicyDenied: &eventv1.PolicyDenied{
			ActionService: k.ActionService, ActionMethod: k.ActionMethod, ActionVerb: k.ActionVerb,
			TargetKind: k.TargetKind, TargetId: k.TargetID, SubReason: k.SubReason, RuleName: k.RuleName,
		}}
	case PolicyCompiled:
		e.Kind = &eventv1.AuthEvent_PolicyCompiled{PolicyCompiled: &eventv1.PolicyCompiled{
			Generation: k.Generation, PolicyCount: k.PolicyCount,
			CompileDurationMs: k.CompileDurationMs, CompiledByPrincipalId: k.CompiledBy,
		}}
	default:
		panic("audit: unknown event kind")
	}
	return r.es.AppendAuth(ctx, e)
}

func (r *Recorder) LoginSucceeded(ctx context.Context, id Identity, k LoginSucceeded) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) LoginFailed(ctx context.Context, id Identity, k LoginFailed) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) Logout(ctx context.Context, id Identity, k Logout) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) SessionRefreshed(ctx context.Context, id Identity, k SessionRefreshed) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) SessionReplayDetected(ctx context.Context, id Identity, k SessionReplayDetected) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) PasswordChanged(ctx context.Context, id Identity, k PasswordChanged) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) PasskeyRegistered(ctx context.Context, id Identity, k PasskeyRegistered) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) PasskeyUnregistered(ctx context.Context, id Identity, k PasskeyUnregistered) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) EnrollmentTokenMinted(ctx context.Context, id Identity, k EnrollmentTokenMinted) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) EnrollmentTokenRedeemed(ctx context.Context, id Identity, k EnrollmentTokenRedeemed) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) TokenMinted(ctx context.Context, id Identity, k TokenMinted) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) TokenRevoked(ctx context.Context, id Identity, k TokenRevoked) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) TokenRejected(ctx context.Context, id Identity, k TokenRejected) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) PolicyDenied(ctx context.Context, id Identity, k PolicyDenied) error {
	return r.emit(ctx, id, k)
}
func (r *Recorder) PolicyCompiled(ctx context.Context, id Identity, k PolicyCompiled) error {
	return r.emit(ctx, id, k)
}

// Domain types per kind, mirroring the proto messages but in plain Go.

type LoginSucceeded struct {
	AuthMethod, UserSlug, SessionID, CredentialID string
}
type LoginFailed struct {
	AuthMethod, AttemptedUserSlug, Reason string
}
type Logout struct{ UserSlug, SessionID string }
type SessionRefreshed struct{ UserSlug, SessionID, NewSessionID string }
type SessionReplayDetected struct {
	UserSlug, SessionID string
	RevokedCount        uint32
}
type PasswordChanged struct{ UserSlug, SetBy string }
type PasskeyRegistered struct{ UserSlug, CredentialID, Label string }
type PasskeyUnregistered struct{ UserSlug, CredentialID, Label string }
type EnrollmentTokenMinted struct {
	UserSlug, Intent string
	ExpiresAt        int64
}
type EnrollmentTokenRedeemed struct{ UserSlug, Intent string }
type TokenMinted struct {
	UserSlug, TokenID, Label, ScopeSummary, IssuedBy string
	TTLSeconds                                       uint32
}
type TokenRevoked struct{ TokenID, RevokedBy, Reason string }
type TokenRejected struct{ TokenIDPrefix, Reason string }
type PolicyDenied struct {
	ActionService, ActionMethod, ActionVerb, TargetKind, TargetID, SubReason, RuleName string
}
type PolicyCompiled struct {
	Generation        uint64
	PolicyCount       uint32
	CompileDurationMs uint32
	CompiledBy        string
}
