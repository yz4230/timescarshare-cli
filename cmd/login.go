package cmd

import (
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/yz4230/timescarshare-cli/internal/session"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Times Car Share in a browser window",
	Long: `Open a Chrome window at the Times Car Share login page, sign in there
(solving the reCAPTCHA if prompted), and save the resulting session cookies.
Later commands reuse that session.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return session.Login(ctx)
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
