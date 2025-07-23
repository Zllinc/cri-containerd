package cmd

import (
	"context"
	"cri-containerd/internal"
	"log"
	"time"

	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/spf13/cobra"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var (
	containerName string
	imageName     string
	namespace     string
)

// 创建容器
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "create a container",
	Long:  `create a container, it can help you to create a container.`,
	Run: func(cmd *cobra.Command, args []string) {
		// if len(args) < 2 {
		// 	log.Fatalf("Usage: %s create -c [container-name] -i [image-name] -n [namespace]", cmd.Use)
		// }
		// containerName = args[0]
		// imageName = args[1]
		// namespace = args[2]

		// 获取server
		server, err := internal.GetServer()
		if err != nil {
			log.Fatalf("failed to get server: %v", err)
		}

		// 拉取镜像
		ctx := namespaces.WithNamespace(context.Background(), namespace)
		image, err := server.PullImage(ctx, imageName)
		if err != nil {
			log.Fatalf("failed to pull image: %v", err)
		}

		// 创建容器
		containerResponse, err := server.CreateContainer(ctx, containerName, image)
		if err != nil {
			log.Fatalf("failed to create container: %v", err)
		}

		// 给容器一些时间执行操作
		log.Default().Println("waiting for container to execute operations")
		time.Sleep(5 * time.Second)

		// 返回容器ID
		log.Default().Println("container id: ", containerResponse.ContainerId)

		// 启动容器
		_, err = server.StartContainer(ctx, &runtimeapi.StartContainerRequest{
			ContainerId: containerResponse.ContainerId,
		})
		if err != nil {
			log.Fatalf("failed to start container: %v", err)
		}
	},
}

// 初始化
func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&containerName, "container-name", "c", "", "container name")
	createCmd.Flags().StringVarP(&imageName, "image-name", "i", "", "image name")
	// 默认使用k8s.io namespace
	createCmd.Flags().StringVarP(&namespace, "namespace", "n", "k8s.io", "namespace")
	createCmd.MarkFlagRequired("container-name")
	createCmd.MarkFlagRequired("image-name")
	// createCmd.MarkFlagRequired("namespace")
}
