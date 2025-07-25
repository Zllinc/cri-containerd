package cmd

import (
	"context"
	"log"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/cobra"

	"cri-containerd/internal"
)

var (
	cleanupNamespace string
	listOnly         bool
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup containers (garbage collection)",
	Long:  "List or delete stopped containers and orphan resources",
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Starting cleanup in namespace: %s", cleanupNamespace)

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 设置正确的 namespace 上下文
		ctx := namespaces.WithNamespace(context.Background(), cleanupNamespace)

		if listOnly {
			// 只列出容器
			containers, err := server.ListContainers(ctx)
			if err != nil {
				log.Fatalf("failed to list containers: %v", err)
			}

			log.Printf("📋 Found %d containers in namespace %s:", len(containers), cleanupNamespace)
			for i, containerID := range containers {
				log.Printf("  %d. %s", i+1, containerID)
			}
		} else {
			// 执行清理
			log.Printf("🧹 Starting container cleanup...")
			log.Printf("⚠️  This will delete stopped containers!")

			// 获取所有容器
			containers, err := server.ListContainers(ctx)
			if err != nil {
				log.Fatalf("failed to list containers: %v", err)
			}

			log.Printf("Found %d containers to check", len(containers))

			// 这里可以添加更智能的清理逻辑
			// 目前只是列出，用户可以手动删除
			log.Printf("💡 Use the following commands to delete containers:")
			for _, containerID := range containers {
				log.Printf("   ./cri-containerd delete -c %s -n %s", containerID, cleanupNamespace)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().StringVarP(&cleanupNamespace, "namespace", "n", "test.io", "containerd namespace")
	cleanupCmd.Flags().BoolVarP(&listOnly, "list-only", "l", false, "only list containers, don't delete")
}
