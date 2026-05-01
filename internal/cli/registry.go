package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/fdatoo/switchyard/internal/registry"
)

func newRegistryCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "registry", Short: "Inspect devices, entities, and drivers"}
	c.AddCommand(newRegistryListCmd(gf))
	c.AddCommand(newRegistryShowCmd(gf))
	return c
}

func newRegistryListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list (devices|entities|drivers)",
		Short: "List registry rows",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			reg, closeDB := loadRegistry(ctx, gf.DataDir)
			defer closeDB()
			switch args[0] {
			case "devices":
				list, err := reg.ListDevices(ctx, registry.DeviceFilter{})
				dieOnError(err)
				t := lgtable.New().Headers("ID", "Driver", "Name").StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
				for _, d := range list {
					t.Row(d.ID, d.DriverInstanceID, d.FriendlyName)
				}
				fmt.Println(t)
			case "entities":
				list, err := reg.ListEntities(ctx, registry.EntityFilter{})
				dieOnError(err)
				t := lgtable.New().Headers("ID", "Type", "Name", "Driver").StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
				for _, e := range list {
					t.Row(EntityID.Render(e.ID), e.EntityType, e.FriendlyName, Dim.Render(e.DriverInstanceID))
				}
				fmt.Println(t)
				fmt.Println(Dim.Render(fmt.Sprintf("%d entities", len(list))))
			case "drivers":
				list, err := reg.ListDriverInstances(ctx)
				dieOnError(err)
				t := lgtable.New().Headers("ID", "Driver", "Status", "Endpoint").StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
				for _, d := range list {
					t.Row(d.ID, d.DriverName, d.Status, d.Endpoint)
				}
				fmt.Println(t)
			default:
				dieOnError(fmt.Errorf("unknown collection %q", args[0]))
			}
		},
	}
}

func newRegistryShowCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show an entity, device, or driver by id",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			reg, closeDB := loadRegistry(ctx, gf.DataDir)
			defer closeDB()
			if e, err := reg.GetEntity(ctx, args[0]); err == nil {
				fmt.Println(Header.Render("Entity"))
				fmt.Printf("  ID:     %s\n  Type:   %s\n  Name:   %s\n  Driver: %s\n  Device: %s\n",
					EntityID.Render(e.ID), e.EntityType, e.FriendlyName, Dim.Render(e.DriverInstanceID), e.DeviceID)
				return
			}
			if d, err := reg.GetDevice(ctx, args[0]); err == nil {
				fmt.Println(Header.Render("Device"))
				fmt.Printf("  ID: %s\n  Driver: %s\n  Name: %s\n", d.ID, Dim.Render(d.DriverInstanceID), d.FriendlyName)
				return
			}
			if di, err := reg.GetDriverInstance(ctx, args[0]); err == nil {
				fmt.Println(Header.Render("Driver"))
				fmt.Printf("  ID: %s\n  Driver: %s\n  Status: %s\n", di.ID, di.DriverName, di.Status)
				return
			}
			dieOnError(fmt.Errorf("no entity/device/driver with id %q", args[0]))
		},
	}
}

func loadRegistry(ctx context.Context, dataDir string) (*registry.Registry, func()) {
	db, err := openReadOnlyDB(ctx, dataDir)
	dieOnError(err)
	reg, err := registry.New(ctx, db)
	dieOnError(err)
	return reg, func() { _ = db.Close() }
}
