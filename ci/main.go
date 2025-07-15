package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ci/cleanup" // adjust to your module path if different
)

func main() {
	var (
		regions        string
		maxRetries     int
		retryBaseDelay int
	)

	rootCmd := &cobra.Command{
		Use:   "cleanup-aws",
		Short: "Cleanup AWS resources produced by CI runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			opts := cleanup.DefaultOptions()

			if regions != "" {
				opts.Regions = strings.Split(regions, ",")
			}
			if maxRetries > 0 {
				opts.MaxRetries = maxRetries
			}
			if retryBaseDelay > 0 {
				// convert int secs to time.Duration
				td := time.Duration(retryBaseDelay) * time.Second
				opts.BaseDelay = td
			}

			if err := cleanup.Run(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&regions, "regions", "r", "", "Comma-separated AWS regions to clean (overrides defaults)")
	rootCmd.Flags().IntVar(&maxRetries, "max-retries", 0, "Maximum number of retries for AWS API calls (0 = library default)")
	rootCmd.Flags().IntVar(&retryBaseDelay, "retry-base-delay", 0, "Base delay in seconds for exponential backoff (0 = default)")

	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
