package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

func newStateCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "state", Short: "Inspect live entity state"}
	c.AddCommand(newStateGetCmd(gf))
	c.AddCommand(newStateDumpCmd(gf))
	return c
}

func newStateGetCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <entity-id>",
		Short: "Print a single entity's current state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewEntityServiceClient(httpClient, base)
			resp, err := svc.Get(cmd.Context(), connect.NewRequest(&v1.GetEntityRequest{Id: args[0]}))
			if err != nil {
				return renderConnectErr(err)
			}
			entity := resp.Msg.GetEntity()
			if entity == nil {
				return fmt.Errorf("entity %q not found", args[0])
			}
			fmt.Printf("%s  %s\n", Header.Render("entity"), EntityID.Render(entity.GetId()))
			if entity.GetFriendlyName() != "" {
				fmt.Printf("  Name: %s\n", entity.GetFriendlyName())
			}
			if entity.GetType() != "" {
				fmt.Printf("  Type: %s\n", Dim.Render(entity.GetType()))
			}
			if state := entity.GetState(); state != nil {
				avail := "no"
				if state.GetAvailable() {
					avail = "yes"
				}
				fmt.Printf("  Available: %s\n", avail)
			}
			if entity.GetState() != nil {
				raw, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(entity.GetState())
				if err != nil {
					return fmt.Errorf("marshal state: %w", err)
				}
				fmt.Printf("  State: %s\n", string(raw))
			}
			return nil
		},
	}
}

func newStateDumpCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "dump",
		Short: "Dump all entity states as JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewEntityServiceClient(httpClient, base)

			out := map[string]any{}
			var pageToken string
			for {
				req := &v1.ListEntitiesRequest{
					Page: &v1.PageRequest{PageSize: 200},
				}
				if pageToken != "" {
					req.Page.PageToken = pageToken
				}
				resp, err := svc.List(cmd.Context(), connect.NewRequest(req))
				if err != nil {
					return renderConnectErr(err)
				}
				for _, entity := range resp.Msg.GetEntities() {
					entry := map[string]any{
						"id":            entity.GetId(),
						"type":          entity.GetType(),
						"friendly_name": entity.GetFriendlyName(),
					}
					if state := entity.GetState(); state != nil {
						entry["available"] = state.GetAvailable()
						raw, _ := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(state)
						var payload any
						_ = json.Unmarshal(raw, &payload)
						entry["state"] = payload
					}
					out[entity.GetId()] = entry
				}
				pageToken = resp.Msg.GetPage().GetNextPageToken()
				if pageToken == "" {
					break
				}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
}
