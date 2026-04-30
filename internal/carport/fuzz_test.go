package carport_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	carportpb "github.com/fdatoo/gohome/gen/gohome/carport/v1alpha1"
)

// FuzzEnvelopeDecode — random bytes should never panic either envelope type.
func FuzzEnvelopeDecode(f *testing.F) {
	seed, _ := proto.Marshal(&carportpb.HostToDriver{
		Kind: &carportpb.HostToDriver_Command{
			Command: &carportpb.Command{CommandId: "x"},
		},
	})
	f.Add(seed)

	seedResp, _ := proto.Marshal(&carportpb.DriverToHost{
		Kind: &carportpb.DriverToHost_Result{
			Result: &carportpb.CommandResult{CommandId: "x", Ok: true},
		},
	})
	f.Add(seedResp)

	f.Fuzz(func(_ *testing.T, data []byte) {
		var h2d carportpb.HostToDriver
		_ = proto.Unmarshal(data, &h2d)
		var d2h carportpb.DriverToHost
		_ = proto.Unmarshal(data, &d2h)
	})
}
