package cmd

import (
	"context"
	"log"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/cobra"

	"cri-containerd/internal"
)

var (
	directContainerName string
	directImageName     string
	directNamespace     string
)

// createDirectCmd represents the create-direct command
var createDirectCmd = &cobra.Command{
	Use:   "create-direct",
	Short: "Create container directly using containerd API in specified namespace",
	Long:  "Create container directly using containerd API, bypassing CRI to avoid kubelet management",
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Creating container directly in namespace: %s", directNamespace)

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 设置正确的 namespace 上下文
		ctx := namespaces.WithNamespace(context.Background(), directNamespace)

		// 直接创建容器
		containerID, err := server.CreateContainerDirectly(ctx, directContainerName, directImageName, directNamespace)
		if err != nil {
			log.Fatalf("failed to create container directly: %v", err)
		}

		log.Printf("Container created successfully!")
		log.Printf("Container ID: %s", containerID)
		log.Printf("Namespace: %s", directNamespace)
		log.Printf("This container should be invisible to kubelet!")
	},
}

func init() {
	rootCmd.AddCommand(createDirectCmd)
	createDirectCmd.Flags().StringVarP(&directContainerName, "container-name", "c", "", "container name")
	createDirectCmd.Flags().StringVarP(&directImageName, "image-name", "i", "", "image name")
	createDirectCmd.Flags().StringVarP(&directNamespace, "namespace", "n", "test.io", "containerd namespace")
	createDirectCmd.MarkFlagRequired("container-name")
	createDirectCmd.MarkFlagRequired("image-name")
}
