// Package main is a minimal scenario-driven driver binary used by integration
// tests. Behavior is selected by TESTDRIVER_MODE, encoded in
// GOHOME_CARPORT_INSTANCE_CONFIG as JSON: {"TESTDRIVER_MODE":"<mode>"}.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
	entitypb "github.com/fdatoo/gohome/gen/gohome/entity/v1"
	eventpb "github.com/fdatoo/gohome/gen/gohome/event/v1"
)

type cfg struct {
	Mode string `json:"TESTDRIVER_MODE"`
}

func main() {
	sock := os.Getenv("GOHOME_CARPORT_SOCKET")
	secret := os.Getenv("GOHOME_CARPORT_SECRET")
	instanceID := os.Getenv("GOHOME_CARPORT_INSTANCE_ID")
	raw := os.Getenv("GOHOME_CARPORT_INSTANCE_CONFIG")
	var c cfg
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	if sock == "" {
		log.Fatal("GOHOME_CARPORT_SOCKET unset")
	}

	ln, err := net.Listen("unix", sock)
	if err != nil {
		log.Fatalf("listen %s: %v", sock, err)
	}
	s := grpc.NewServer()
	carportpb.RegisterDriverServer(s, &server{
		mode:           c.Mode,
		expectedSecret: secret,
		instanceID:     instanceID,
	})
	log.Printf("testdriver ready: mode=%s sock=%s instance=%s", c.Mode, sock, instanceID)
	if err := s.Serve(ln); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

type server struct {
	carportpb.UnimplementedDriverServer
	mode           string
	expectedSecret string
	instanceID     string
}

func (s *server) Handshake(_ context.Context, req *carportpb.HandshakeRequest) (*carportpb.HandshakeResponse, error) {
	switch s.mode {
	case "bad_secret":
		return nil, status.Error(codes.Unauthenticated, "bad secret")
	case "bad_protocol_version":
		return &carportpb.HandshakeResponse{ProtocolVersion: "v99"}, nil
	case "slow_handshake":
		time.Sleep(10 * time.Second)
		return &carportpb.HandshakeResponse{ProtocolVersion: "v1alpha1"}, nil
	}
	if req.GetHandshakeSecret() != s.expectedSecret {
		return nil, status.Error(codes.Unauthenticated, "secret mismatch")
	}
	resp := &carportpb.HandshakeResponse{
		ProtocolVersion: "v1alpha1",
		Manifest: &carportpb.DriverManifest{
			Name:            "testdriver",
			Version:         "0.0.0",
			ProtocolVersion: "v1alpha1",
		},
	}
	if s.mode == "repeat_register" {
		resp.InitialEntities = []*eventpb.EntityRegistered{{
			DriverInstanceId: s.instanceID,
			EntityType:       "light",
			FriendlyName:     "test_light",
			Capabilities:     &entitypb.Attributes{},
		}}
	}
	return resp, nil
}

func (s *server) Run(srv carportpb.Driver_RunServer) error {
	switch s.mode {
	case "crash_after_handshake":
		time.Sleep(100 * time.Millisecond)
		os.Exit(2)
	case "crash_mid_stream":
		time.Sleep(250 * time.Millisecond)
		os.Exit(2)
	case "chatty":
		for i := 0; i < 1000; i++ {
			if err := srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_StateChanged{
					StateChanged: &eventpb.StateChanged{Attributes: &entitypb.Attributes{}},
				},
			}); err != nil {
				return err
			}
		}
	}

	for {
		in, err := srv.Recv()
		if err != nil {
			return err
		}
		switch k := in.GetKind().(type) {
		case *carportpb.HostToDriver_Command:
			if s.mode == "hang_on_command" {
				time.Sleep(time.Hour)
				continue
			}
			_ = srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Result{
					Result: &carportpb.CommandResult{
						CommandId: k.Command.GetCommandId(),
						Ok:        true,
					},
				},
			})
		case *carportpb.HostToDriver_Ping:
			_ = srv.Send(&carportpb.DriverToHost{
				Kind: &carportpb.DriverToHost_Pong{Pong: &carportpb.Heartbeat{TsUnixMs: time.Now().UnixMilli()}},
			})
		}
	}
}

func (s *server) Health(_ context.Context, _ *carportpb.HealthRequest) (*carportpb.HealthResponse, error) {
	return &carportpb.HealthResponse{Ok: true}, nil
}

func (s *server) Shutdown(_ context.Context, _ *carportpb.ShutdownRequest) (*carportpb.ShutdownResponse, error) {
	if s.mode == "hang_on_shutdown" {
		time.Sleep(time.Hour)
	}
	return &carportpb.ShutdownResponse{Acknowledged: true}, nil
}
