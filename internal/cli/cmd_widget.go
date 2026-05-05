package cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newWidgetCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "widget", Short: "Manage widget packs"}
	cmd.AddCommand(newWidgetInstallCmd(gf))
	cmd.AddCommand(newWidgetListCmd(gf))
	cmd.AddCommand(newWidgetUninstallCmd(gf))
	return cmd
}

func newWidgetInstallCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "install <oci-ref>",
		Short: "Install a widget pack from an OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := dialWidgetPack(cmd.Context(), gf)
			if err != nil {
				return err
			}
			resp, err := client.Install(cmd.Context(), connect.NewRequest(&v1.InstallWidgetPackRequest{Ref: args[0]}))
			if err != nil {
				return renderConnectErr(err)
			}
			renderInstalled(resp.Msg.GetPack())
			return nil
		},
	}
}

func newWidgetListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed widget packs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := dialWidgetPack(cmd.Context(), gf)
			if err != nil {
				return err
			}
			resp, err := client.List(cmd.Context(), connect.NewRequest(&v1.ListWidgetPacksRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			packs := resp.Msg.GetPacks()
			if len(packs) == 0 {
				fmt.Println(Dim.Render("no packs installed"))
				return nil
			}
			fmt.Printf("%s\t%s\t%s\t%s\n",
				Header.Render("NAME"), Header.Render("VERSION"),
				Header.Render("SIG"), Header.Render("CLASSES"))
			for _, p := range packs {
				fmt.Printf("%s\t%s\t%s\t%v\n",
					PackName.Render(p.GetName()),
					PackVersion.Render(p.GetVersion()),
					sigBadge(p.GetSignature()),
					p.GetClasses())
			}
			return nil
		},
	}
}

func newWidgetUninstallCmd(gf *globalFlags) *cobra.Command {
	var version string
	var force bool
	cmd := &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a widget pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := dialWidgetPack(cmd.Context(), gf)
			if err != nil {
				return err
			}
			versions := []string{version}
			if version == "" {
				resp, err := client.List(cmd.Context(), connect.NewRequest(&v1.ListWidgetPacksRequest{}))
				if err != nil {
					return renderConnectErr(err)
				}
				versions = nil
				for _, p := range resp.Msg.GetPacks() {
					if p.GetName() == args[0] {
						versions = append(versions, p.GetVersion())
					}
				}
				if len(versions) == 0 {
					return fmt.Errorf("no installed versions of %q", args[0])
				}
			}
			for _, v := range versions {
				_, err := client.Uninstall(cmd.Context(), connect.NewRequest(&v1.UninstallWidgetPackRequest{
					Name: args[0], Version: v, Force: force,
				}))
				if err != nil {
					return renderConnectErr(err)
				}
				fmt.Printf("%s %s@%s\n", Success.Render("uninstalled"), args[0], v)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "specific version (default: all installed)")
	cmd.Flags().BoolVar(&force, "force", false, "uninstall even if dashboards reference the pack's classes")
	return cmd
}

func dialWidgetPack(ctx context.Context, gf *globalFlags) (switchyardv1alpha1connect.WidgetPackServiceClient, error) {
	ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
	httpClient, base, err := Dial(ctx, ep)
	if err != nil {
		return nil, err
	}
	return switchyardv1alpha1connect.NewWidgetPackServiceClient(httpClient, base), nil
}

func renderInstalled(p *v1.InstalledPack) {
	if p == nil {
		return
	}
	fmt.Printf("%s %s@%s %s\n",
		Success.Render("installed"),
		PackName.Render(p.GetName()),
		PackVersion.Render(p.GetVersion()),
		sigBadge(p.GetSignature()))
	if p.GetSignerIdentity() != "" {
		fmt.Printf("  signer: %s\n", Dim.Render(p.GetSignerIdentity()))
	}
	fmt.Printf("  classes: %v\n", p.GetClasses())
}

func sigBadge(s v1.SignatureStatus) string {
	switch s {
	case v1.SignatureStatus_SIGNATURE_VERIFIED:
		return PackVerified.Render("✓ verified")
	case v1.SignatureStatus_SIGNATURE_UNSIGNED:
		return PackUnsigned.Render("⚠ unsigned")
	case v1.SignatureStatus_SIGNATURE_INVALID:
		return PackExpired.Render("✗ invalid")
	default:
		return Dim.Render("?")
	}
}
