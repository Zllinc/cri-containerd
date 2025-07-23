package cmd

import (
	"context"
	"log"

	"cri-containerd/internal"

	"github.com/spf13/cobra"
)

var deleteContainerID string

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete a container",
	Long:  `delete a container, it can help you to delete a container.`,
	Run: func(cmd *cobra.Command, args []string) {
		// if len(args) < 1 {
		// 	log.Fatalf("Usage: %s delete -c [container-ID]", cmd.Use)
		// }

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 删除容器
		err = server.DeleteContainer(context.Background(), containerID)
		if err != nil {
			log.Fatalf("failed to delete container: %v", err)
		}
		log.Default().Printf("deleted container: %s successfully! \n", containerID)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteContainerID, "container-id", "c", "", "container id")
	deleteCmd.MarkFlagRequired("container-id")
}
