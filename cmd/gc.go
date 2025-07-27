package cmd

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	// "github.com/containerd/containerd/v2/pkg/namespaces"

	"cri-containerd/internal"
)

var(
	gcNamespace string
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Garbage collect stopped containers and orphaned resources",
	Long: `Garbage collect (gc) will clean up:
		1. Orphaned containers (containers without tasks)
		2. Non-running containers with devbox.sealos.io/content-id label
		3. Associated snapshots and resources`,
	Run: func(cmd *cobra.Command, args []string) {
		// 创建 Server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("Failed to create server: %v", err)
		}

		// 设置正确的 namespace 上下文
		// ctx := namespaces.WithNamespace(context.Background(), gcNamespace)

		// 执行垃圾回收
		err = server.CleanupOrphanContainers(context.Background(), gcNamespace)
		if err != nil {
			log.Fatalf("Failed to cleanup containers: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(gcCmd)
	gcCmd.Flags().StringVarP(&gcNamespace, "namespace", "n", "test.io","containerd namespace")
	// 添加命令行参数
	// gcCmd.Flags().StringP("namespace", "n", "default", "Namespace to perform garbage collection in")
}
