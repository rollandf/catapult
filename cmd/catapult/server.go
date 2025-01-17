package cmd

import (
	"github.com/PUMATeam/catapult/pkg/api"
	"github.com/spf13/cobra"
)

var port int

// restCmd represents the rest command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start catapult server",
	Long:  `Start catapult server`,
	Run: func(cmd *cobra.Command, args []string) {
		api.Start(port)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().IntVarP(&port, "port", "p", 8888, "Port for which to listen")
}
