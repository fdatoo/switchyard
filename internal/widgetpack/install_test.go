package widgetpack_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fdatoo/switchyard/internal/widgetpack"
)

// TestInstaller_Install_BadRef is a smoke test for the Installer's request
// validation. Full end-to-end coverage of the install pipeline lives in the
// Task 15 integration test, which exercises a real OCI registry, signer,
// and on-disk store.
func TestInstaller_Install_BadRef(t *testing.T) {
	inst := widgetpack.NewInstaller(nil, nil, nil, nil, "", nil)
	_, err := inst.Install(context.Background(), widgetpack.InstallRequest{Ref: ""})
	var fe *widgetpack.FailureError
	if !errors.As(err, &fe) || fe.Reason != widgetpack.ReasonBadRef {
		t.Errorf("err = %v, want FailureError{Reason: bad_ref}", err)
	}
}
