package widgetpack

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

// Service implements WidgetPackServiceHandler.
type Service struct {
	installer *Installer
	store     *Store
}

// NewService wires the Connect handler. installer may be nil when only List is
// needed (e.g. read-only frontends), but Install and Uninstall will panic.
func NewService(installer *Installer, store *Store) *Service {
	return &Service{installer: installer, store: store}
}

// Compile-time assertion: Service must satisfy WidgetPackServiceHandler.
var _ switchyardv1alpha1connect.WidgetPackServiceHandler = (*Service)(nil)

func (s *Service) Install(ctx context.Context, req *connect.Request[v1.InstallWidgetPackRequest]) (*connect.Response[v1.InstallWidgetPackResponse], error) {
	pack, err := s.installer.Install(ctx, InstallRequest{Ref: req.Msg.GetRef()})
	if err != nil {
		return nil, mapInstallErr(err)
	}
	return connect.NewResponse(&v1.InstallWidgetPackResponse{Pack: toProto(pack)}), nil
}

func (s *Service) List(ctx context.Context, _ *connect.Request[v1.ListWidgetPacksRequest]) (*connect.Response[v1.ListWidgetPacksResponse], error) {
	packs, err := s.store.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*v1.InstalledPack, 0, len(packs))
	for i := range packs {
		out = append(out, toProto(&packs[i]))
	}
	return connect.NewResponse(&v1.ListWidgetPacksResponse{Packs: out}), nil
}

func (s *Service) Uninstall(ctx context.Context, req *connect.Request[v1.UninstallWidgetPackRequest]) (*connect.Response[v1.UninstallWidgetPackResponse], error) {
	if err := s.installer.Uninstall(ctx, req.Msg.GetName(), req.Msg.GetVersion(), req.Msg.GetForce()); err != nil {
		if errors.Is(err, ErrPackNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(&v1.UninstallWidgetPackResponse{}), nil
}

func (s *Service) Watch(ctx context.Context, _ *connect.Request[v1.WatchWidgetPacksRequest], stream *connect.ServerStream[v1.WidgetPackEvent]) error {
	ch := make(chan WatchEvent, 16)
	unsub := s.store.Subscribe(ch)
	defer unsub()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-ch:
			if err := stream.Send(eventToProto(ev)); err != nil {
				return err
			}
		}
	}
}

// toProto converts a Store InstalledPack to its proto representation.
func toProto(p *InstalledPack) *v1.InstalledPack {
	if p == nil {
		return nil
	}
	return &v1.InstalledPack{
		Name:           p.Name,
		Version:        p.Version,
		Sha256:         p.SHA256,
		Signature:      sigToProto(p.SignatureStatus),
		SignerIdentity: p.SignerIdentity,
		Classes:        p.Classes,
		BundleUrl:      "/widgets/" + p.Name + "/" + p.Version + "/bundle.js?h=" + p.SHA256,
		Description:    p.Description,
		Homepage:       p.Homepage,
		License:        p.License,
		InstalledAt:    timestamppb.New(p.InstalledAt),
	}
}

// sigToProto maps the Store's string status onto the proto enum.
// Status values are "verified", "unsigned", "invalid".
func sigToProto(s string) v1.SignatureStatus {
	switch s {
	case "verified":
		return v1.SignatureStatus_SIGNATURE_VERIFIED
	case "unsigned":
		return v1.SignatureStatus_SIGNATURE_UNSIGNED
	case "invalid":
		return v1.SignatureStatus_SIGNATURE_INVALID
	default:
		return v1.SignatureStatus_SIGNATURE_UNKNOWN
	}
}

// eventToProto converts a WatchEvent to its proto representation.
func eventToProto(ev WatchEvent) *v1.WidgetPackEvent {
	if ev.Installed != nil {
		return &v1.WidgetPackEvent{Kind: &v1.WidgetPackEvent_Installed{Installed: toProto(ev.Installed)}}
	}
	if ev.Uninstalled != nil {
		return &v1.WidgetPackEvent{Kind: &v1.WidgetPackEvent_Uninstalled{
			Uninstalled: &v1.UninstalledPack{Name: ev.Uninstalled.Name, Version: ev.Uninstalled.Version},
		}}
	}
	return &v1.WidgetPackEvent{}
}

// mapInstallErr translates Installer errors to Connect status codes.
func mapInstallErr(err error) error {
	var fe *FailureError
	if errors.As(err, &fe) {
		switch fe.Reason {
		case ReasonBadRef:
			return connect.NewError(connect.CodeInvalidArgument, err)
		case ReasonRegistryUnreachable:
			return connect.NewError(connect.CodeUnavailable, err)
		case ReasonAlreadyExists:
			return connect.NewError(connect.CodeAlreadyExists, err)
		case ReasonBadArtifact, ReasonSignatureInvalid, ReasonHashMismatch,
			ReasonSDKIncompatible, ReasonClassCollision, ReasonManifestInvalid:
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}
