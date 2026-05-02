package cli

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newSnapshotCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "snapshot", Short: "Create and list snapshots"}
	c.AddCommand(newSnapshotCreateCmd(gf))
	c.AddCommand(newSnapshotListCmd(gf))
	return c
}

func newSnapshotCreateCmd(gf *globalFlags) *cobra.Command {
	var owner, reason string
	c := &cobra.Command{
		Use:   "create",
		Short: "Trigger an immediate snapshot via the daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewSystemServiceClient(httpClient, base)
			resp, err := svc.CreateSnapshot(cmd.Context(), connect.NewRequest(&v1.CreateSnapshotRequest{
				Owner:  owner,
				Reason: reason,
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			msg := resp.Msg
			createdAt := ""
			if msg.GetCreatedAt() != nil {
				createdAt = msg.GetCreatedAt().AsTime().Format(time.RFC3339)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "snapshot: cursor %d at %s\n",
				msg.GetCursor(), createdAt)
			return nil
		},
	}
	c.Flags().StringVar(&owner, "owner", "state_cache", "owner projector")
	c.Flags().StringVar(&reason, "reason", "manual", "reason recorded in snapshot meta")
	return c
}

func newSnapshotListCmd(gf *globalFlags) *cobra.Command {
	var owner string
	c := &cobra.Command{
		Use:   "list",
		Short: "List snapshots stored in the DB",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()

			query := `SELECT position, ts, owner, encoding, LENGTH(state) FROM snapshots`
			args := []any{}
			if owner != "" {
				query += ` WHERE owner = ?`
				args = append(args, owner)
			}
			query += ` ORDER BY position DESC LIMIT 50`
			rows, err := db.QueryContext(context.Background(), query, args...)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()

			t := lgtable.New().
				Headers("Position", "Owner", "Encoding", "Size", "Time").
				StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
			for rows.Next() {
				var pos, tsNanos, size int64
				var o, enc string
				if err := rows.Scan(&pos, &tsNanos, &o, &enc, &size); err != nil {
					return err
				}
				t.Row(
					fmt.Sprint(pos),
					o,
					enc,
					fmt.Sprintf("%.1f KB", float64(size)/1024),
					time.Unix(0, tsNanos).Format("2006-01-02 15:04:05"),
				)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), t)
			return nil
		},
	}
	c.Flags().StringVar(&owner, "owner", "", "filter by owner")
	return c
}
