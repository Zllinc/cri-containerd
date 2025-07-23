package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cri-containerd",
	Short: "cri-containerd is a tool for managing containerd",
	Long:  `cri-containerd is a tool for managing containerd, it can help you to create, start, stop, and delete containers.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
