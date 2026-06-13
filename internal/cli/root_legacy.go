//go:build legacy

package cli

func init() {
	rootCmd.AddCommand(newAppServerCmd())
}
