package cmd

import (
	"context"
	"cri-containerd/internal"
	"log"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/cobra"
)

var (
	committedImageName string
	containerID        string
	commitNamespace    string
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "commit a container",
	Long:  `commit a container, it can help you to commit a container.`,
	Run: func(cmd *cobra.Command, args []string) {
		// if len(args) < 2 {
		// 	log.Fatalf("Usage: %s commit -c [container-ID] -i [image-name]", cmd.Use)
		// }

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 提交容器为新镜像
		ctx := namespaces.WithNamespace(context.Background(), commitNamespace)
		err = server.CommitContainer(ctx, containerID, committedImageName)
		if err != nil {
			log.Fatalf("failed to commit container: %v", err)
		}

		log.Default().Printf("✅ Committed image: %s successfully! \n", committedImageName)
	},
}

func init() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringVarP(&containerID, "container-id", "c", "", "container id")
	commitCmd.Flags().StringVarP(&committedImageName, "image-name", "i", "", "committed image name")
	commitCmd.Flags().StringVarP(&commitNamespace, "namespace", "n", "test.io", "containerd namespace")
	commitCmd.MarkFlagRequired("container-id")
	commitCmd.MarkFlagRequired("image-name")

}
