package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/cobra"

	"cri-containerd/internal"
)

var (
	inspectContainerName string
	inspectNamespace     string
	showLabelsOnly       bool
)

// inspectCmd represents the inspect command
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect container annotations and details",
	Long:  "Inspect container to see annotations, labels and other detailed information",
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Inspecting container: %s in namespace: %s", inspectContainerName, inspectNamespace)

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 设置正确的 namespace 上下文
		ctx := namespaces.WithNamespace(context.Background(), inspectNamespace)

		if showLabelsOnly {
			// 只显示 annotations/labels
			annotations, err := server.GetContainerAnnotations(ctx, inspectContainerName)
			if err != nil {
				log.Fatalf("failed to get container annotations: %v", err)
			}

			fmt.Printf("📋 Container Annotations/Labels:\n")
			for key, value := range annotations {
				fmt.Printf("  %s: %s\n", key, value)
			}
		} else {
			// 显示完整信息
			info, err := server.GetContainerInfo(ctx, inspectContainerName)
			if err != nil {
				log.Fatalf("failed to get container info: %v", err)
			}

			// 格式化输出
			jsonData, err := json.MarshalIndent(info, "", "  ")
			if err != nil {
				log.Fatalf("failed to marshal container info: %v", err)
			}

			fmt.Printf("🔍 Container Info:\n%s\n", string(jsonData))
		}
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
	inspectCmd.Flags().StringVarP(&inspectContainerName, "container-name", "c", "", "container name to inspect")
	inspectCmd.Flags().StringVarP(&inspectNamespace, "namespace", "n", "test.io", "containerd namespace")
	inspectCmd.Flags().BoolVarP(&showLabelsOnly, "labels-only", "l", false, "show only annotations/labels")
	inspectCmd.MarkFlagRequired("container-name")
}
