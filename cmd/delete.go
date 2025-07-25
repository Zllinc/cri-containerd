package cmd

import (
	"context"
	"log"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/cobra"

	"cri-containerd/internal"
)

var (
	deleteContainerName string
	deleteNamespace     string
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a container",
	Long:  "Delete a container and its associated resources",
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Deleting container: %s in namespace: %s", deleteContainerName, deleteNamespace)

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 设置正确的 namespace 上下文
		ctx := namespaces.WithNamespace(context.Background(), deleteNamespace)

		// 删除容器
		err = server.DeleteContainerDirectly(ctx, deleteContainerName)
		if err != nil {
			log.Fatalf("failed to delete container: %v", err)
		}

		log.Printf("✅ Container %s deleted successfully!", deleteContainerName)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteContainerName, "container-name", "c", "", "container name to delete")
	deleteCmd.Flags().StringVarP(&deleteNamespace, "namespace", "n", "test.io", "containerd namespace")
	deleteCmd.MarkFlagRequired("container-name")
}
