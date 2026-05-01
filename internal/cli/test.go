package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newTestCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "test <file.star | dir/>",
		Short: "Run test_* functions in Starlark test files against the running daemon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := collectTestFiles(args[0])
			if err != nil {
				return err
			}

			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewScriptServiceClient(httpClient, base)

			var totalPass, totalFail int
			for _, f := range files {
				p, fa, err := runConnectTestFile(cmd.Context(), svc, f, cmd.OutOrStdout())
				totalPass += p
				totalFail += fa
				if err != nil {
					fmt.Fprintln(os.Stderr, Error.Render("error: ")+err.Error())
				}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%s %d passed, %d failed\n",
				Dim.Render("summary:"), totalPass, totalFail)
			if totalFail > 0 {
				_, _ = fmt.Fprintln(os.Stderr, Error.Render("FAIL"))
				os.Exit(1)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), Success.Render("ok"))
			return nil
		},
	}
}

func collectTestFiles(target string) ([]string, error) {
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{target}, nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.star") {
			files = append(files, filepath.Join(target, e.Name()))
		}
	}
	return files, nil
}

// runConnectTestFile streams test results for a single path via RunTests.
// Returns (passCount, failCount, error).
func runConnectTestFile(ctx context.Context, svc switchyardv1alpha1connect.ScriptServiceClient, filePath string, out io.Writer) (int, int, error) {
	stream, err := svc.RunTests(ctx, connect.NewRequest(&v1.RunTestsRequest{Path: filePath}))
	if err != nil {
		return 0, 0, renderConnectErr(err)
	}
	defer func() { _ = stream.Close() }()

	var pass, fail int
	for stream.Receive() {
		msg := stream.Msg()
		if ev := msg.GetEvent(); ev != nil {
			if ev.GetOutcome() == "ok" {
				_, _ = fmt.Fprintf(out, "%s %s\n", Success.Render("--- PASS:"), ev.GetName())
				pass++
			} else {
				_, _ = fmt.Fprintf(out, "%s %s\n", Error.Render("--- FAIL:"), ev.GetName())
				if detail := ev.GetDetail(); detail != "" {
					_, _ = fmt.Fprintf(out, "    %s\n", EntityID.Render(detail))
				}
				fail++
			}
		}
		// heartbeats: ignore
	}
	if err := stream.Err(); err != nil {
		return pass, fail, renderConnectErr(err)
	}
	return pass, fail, nil
}
