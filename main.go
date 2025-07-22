package main

import (
	"context"
	"fmt"
	"log"
	"os"

	// "syscall"
	"strings"
	"time"

	"github.com/google/uuid"

	// "github.com/containerd/containerd"
	// "github.com/containerd/containerd/cio"
	// "github.com/containerd/containerd/errdefs"
	// "github.com/containerd/containerd/v2/namespaces"
	// "github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"

	// "github.com/containerd/nerdctl/v2/pkg/api/types"
	// "github.com/containerd/nerdctl/v2/pkg/imgutil/commit"
	imageutil "github.com/labring/cri-shim/pkg/image"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	// "github.com/containerd/containerd/v2/core/containers"
)

type Server struct {
	runtimeServiceClient runtimeapi.RuntimeServiceClient // 与containerd进行交互的client
	containerdClient     *client.Client                  // 也是与containerd进行交互的client
	imageClient          imageutil.ImageInterface        // 与image进行交互的client
}

const (
	namespace = "k8s.io"
	address   = "unix:///var/run/containerd/containerd.sock"
	// fStdout = os.Stdout // 或 os.DevNull
)

func main() {
	// 连接到containerd守护进程
	// 1. 获取server
	server, err := getServer()
	if err != nil {
		log.Fatalf("failed to connect to containerd: %v", err)
	}
	defer server.containerdClient.Close()

	// 2. 创建context
	ctx := namespaces.WithNamespace(context.Background(), namespace)

	// 要使用的镜像名
	imageName := "docker.io/library/busybox:latest"
	containerName := "test-container"
	committedImageName := "committed-busybox:v1"

	// 拉取镜像
	image, err := server.pullImage(ctx, imageName)
	if err != nil {
		log.Fatalf("failed to pull image: %v", err)
	}

	// 创建容器
	containerResponse, err := server.createContainer(ctx, containerName, image)
	if err != nil {
		log.Fatalf("failed to create container: %v", err)
	}

	// 给容器一些时间执行操作
	log.Default().Println("waiting for container to execute operations")
	time.Sleep(5 * time.Second)

	// 启动容器
	_, err = server.startContainer(ctx, &runtimeapi.StartContainerRequest{
		ContainerId: containerResponse.ContainerId,
	})
	if err != nil {
		log.Fatalf("failed to start container: %v", err)
	}

	// 提交容器为新镜像
	err = server.commitContainer(ctx, containerResponse.ContainerId, committedImageName)
	if err != nil {
		log.Fatalf("failed to commit container: %v", err)
	}
	log.Default().Printf("committed image: %s successfully! \n", committedImageName)

	// 删除容器
	err = server.deleteContainer(ctx, containerResponse.ContainerId)
	if err != nil {
		log.Fatalf("failed to delete container: %v", err)
	}
}

// 获取Server
func getServer() (*Server, error) {
	log.Default().Printf("connecting to containerd: %s ...\n", address)
	// 1. 创建 gRPC 连接
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	// 2. 创建 containerd 客户端
	containerdClient, err := client.NewWithConn(conn, client.WithDefaultNamespace(namespace))
	if err != nil {
		return nil, err
	}

	// 3. 创建client
	client := runtimeapi.NewRuntimeServiceClient(conn)

	// 4. 创建imageClient
	fStdout := os.Stdout // 或 os.DevNull

	imageClient, err := imageutil.NewImageInterface(namespace, address, fStdout)
	if err != nil {
		panic(err)
	}

	return &Server{
		runtimeServiceClient: client,
		containerdClient:     containerdClient,
		imageClient:          imageClient,
	}, nil
}

// 拉取镜像：拉取镜像本质还是使用的containerd.Client.Pull()
func (s *Server) pullImage(ctx context.Context, imageName string) (client.Image, error) {
	log.Default().Printf("pulling image: %s ...\n", imageName)
	image, err := s.containerdClient.Pull(ctx, imageName, client.WithPullUnpack)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %s, err: %v", imageName, err)
	}
	log.Default().Printf("pulled image: %s successfully! \n", image.Name())
	return image, nil
}

// 创建PodSandbox: 创建PodSandbox本质是使用的runtimeServiceClient.RunPodSandbox()
func (s *Server) runPodSandbox(ctx context.Context, request *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	log.Default().Println("Doing run pod sandbox request", "request", request)
	return s.runtimeServiceClient.RunPodSandbox(ctx, request)
}

// 创建容器: 调用了runPodSandbox
func (s *Server) createContainer(ctx context.Context, containerName string, image client.Image) (*runtimeapi.CreateContainerResponse, error) {
	// 先创建PodSandbox
	// podName := "test-pod"
	cgroupParent := "system.slice"

	// 生成唯一且安全的 Pod 名称和 content-id
	podUUID := uuid.New().String()
	podName := fmt.Sprintf("test-pod-%s", strings.ReplaceAll(podUUID, "-", "")) // 移除短横线
	contentID := fmt.Sprintf("content-%s", podUUID)

	podSandboxReq := &runtimeapi.RunPodSandboxRequest{
		Config: &runtimeapi.PodSandboxConfig{
			Metadata: &runtimeapi.PodSandboxMetadata{
				Name:      podName,
				Namespace: "default",
				Uid:       podUUID,
				Attempt:   1,
			},
			Hostname:     "my-pod",
			LogDirectory: "/var/log/pods/my-pod",
			Labels: map[string]string{
				"app": "my-app",
			},
			Annotations: map[string]string{
				"devbox.sealos.io/content-id": contentID,
				"description":                 "my-pod-description",
			},
			Linux: &runtimeapi.LinuxPodSandboxConfig{
				CgroupParent: cgroupParent, // 格式为 slice:prefix:name
			},
		},
	}

	response, err := s.runPodSandbox(ctx, podSandboxReq)
	if err != nil {
		return nil, fmt.Errorf("failed to run pod sandbox: %v", err)
	}

	// 验证 PodSandboxId 有效性
	log.Default().Println("PodSandboxId: ", response.PodSandboxId)
	if response.PodSandboxId == "" {
		return nil, fmt.Errorf("invalid PodSandboxId: empty")
	}

	// 创建容器
	containerReq := &runtimeapi.CreateContainerRequest{
		PodSandboxId:  response.PodSandboxId,
		SandboxConfig: podSandboxReq.Config,
		Config: &runtimeapi.ContainerConfig{
			Metadata: &runtimeapi.ContainerMetadata{
				Name:    containerName,
				Attempt: 1,
			},
			Image: &runtimeapi.ImageSpec{
				Image: image.Name(),
			},
			Command:    []string{"/bin/sh", "-c", "while true; do echo 'Hello, World!'; sleep 5; done"},
			WorkingDir: "/root",
			Stdin:      true,
			StdinOnce:  true,
			LogPath:    "/var/log/my-container.log",
		},
	}
	responseContainer, err := s.runtimeServiceClient.CreateContainer(ctx, containerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %v", err)
	}

	return responseContainer, nil
}

// 运行容器
func (s *Server) startContainer(ctx context.Context, request *runtimeapi.StartContainerRequest) (*runtimeapi.StartContainerResponse, error) {
	log.Default().Println("Doing start container request", "request", request)
	return s.runtimeServiceClient.StartContainer(ctx, request)
}

// commit容器
func (s *Server) commitContainer(ctx context.Context, containerID, committedImageName string) error {
	return s.imageClient.Commit(ctx, committedImageName, containerID, true)
}

// delete容器
func (s *Server) deleteContainer(ctx context.Context, containerID string) error {
	req := &runtimeapi.RemoveContainerRequest{
		ContainerId: containerID,
	}
	_, err := s.runtimeServiceClient.RemoveContainer(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete container: %v", err)
	}
	return nil
}

// // 创建并运行容器
// func createAndStartContainer(ctx context.Context, client *client.Client, containerName string, image client.Image) (client.Container, error) {
// 	fmt.Printf("creating and starting container: %s ...\n", containerName)
// 	container, err := client.NewContainer(
// 		ctx, containerName,
// 		containerd.WithImage(image),
// 		containerd.WithNewSnapshot(fmt.Sprintf("snapshot-%s", containerName), image),
// 		containerd.WithNewSpec(oci.WithImageConfig(image)),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create container: %s, err: %v", containerName, err)
// 	}

// 	// 创建并且启动任务
// 	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create task: %s, err: %v", containerName, err)
// 	}
// 	defer task.Delete(ctx)

// 	// 启动容器
// 	if err := task.Start(ctx); err != nil {
// 		return nil, fmt.Errorf("failed to start container: %s, err: %v", containerName, err)
// 	}

// 	// 等待容器启动
// 	for i := 0; i < 10; i++ {
// 		status, err := task.Status(ctx)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get container status: %s, err: %v", containerName, err)
// 		}
// 		if status.Status == containerd.Running {
// 			break
// 		}
// 		time.Sleep(500 * time.Millisecond)
// 	}

// 	// 返回容器和任务
// 	fmt.Printf("container: %s started successfully! \n", containerName)
// 	return container, nil
// }

// // 提交容器为新镜像
// func commitContainer(ctx context.Context, client *client.Client, containerName string, committedImageName string) (client.Image, error) {
// 	fmt.Printf("committing container: %s to image: %s ...\n", containerName, committedImageName)

// 	runtimeServiceClient := runtimeApi.NewRuntimeServiceClient(client)
// 	// 获取容器状态

// 	// 获取容器的信息，如registry源等

// 	// 拉取基础镜像

// 	// 执行镜像commit操作：s.imageClient.Commit
// 	// 1. 调用container.Commit（“github.com/containerd/nerdctl/v2/pkg/cmd/container”）

// 	// 容器提交选项
// 	options := types.ContainerCommitOptions{
// 		Author:  "Cunzili",
// 		Message: "committed via API",
// 		Pause:   true,
// 		Change:  []string{},
// 		GOptions: types.GlobalCommandOptions{
// 			Namespace:        "default",
// 			Address:          "/run/containerd/containerd.sock",
// 			DataRoot:         "/var/lib/containerd",
// 			InsecureRegistry: true,
// 		},
// 	}

// 	container, err := client.LoadContainer(ctx, containerName)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to load container: %w", err)
// 	}

// 	commit.Commit(ctx, client, container, options, options.GOptions)

// 	snapshotter := os.Getenv("CONTAINERD_SNAPSHOTTER")
// 	if snapshotter == "" {
// 		snapshotter = "overlayfs" // 默认使用 overlayfs
// 	}

// 	// 获取容器的当前状态
// 	task, err := container.Task(ctx, nil)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get container task: %w", err)
// 	}

// 	// 确保容器已停止
// 	if status, err := task.Status(ctx); err == nil && status.Status == containerd.Running {
// 		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
// 			return nil, fmt.Errorf("failed to stop container: %w", err)
// 		}
// 		// 等待容器停止
// 		if _, err := task.Wait(ctx); err != nil {
// 			return nil, fmt.Errorf("failed to wait for container to stop: %w", err)
// 		}
// 	}

// 	// 创建只读快照视图（使用容器ID作为名称）
// 	if _, err := client.SnapshotService(snapshotter).View(ctx, container.ID(), container.ID()); err != nil {
// 		return nil, fmt.Errorf("failed to create snapshot view: %w", err)
// 	}
// 	defer client.SnapshotService(snapshotter).Remove(ctx, container.ID())

// 	// Commit容器的文件系统到新镜像
// 	if err := client.SnapshotService(snapshotter).Commit(ctx, committedImageName, container.ID()); err != nil {
// 		return nil, fmt.Errorf("failed to commit container: %w", err)
// 	}

// 	// 从名称加载新创建的镜像
// 	image, err := client.GetImage(ctx, committedImageName)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get new image: %w", err)
// 	}

// 	// 创建并保存镜像配置
// 	platform := image.Platform()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get image platform: %w", err)
// 	}
// 	img, err := containerd.NewImageWithPlatform(client, image, platform)
// 	if err != nil {
// 		return fmt.Errorf("创建镜像配置失败: %w", err)
// 	}

// 	// 标记镜像
// 	if err := img.Label(ctx, map[string]string{
// 		"created-by": "containerd-sample",
// 	}); err != nil {
// 		return fmt.Errorf("标记镜像失败: %w", err)
// 	}

// 	fmt.Printf("成功创建新镜像: %s\n", imageName)
// 	return nil

// 	// // 获取快照服务
// 	// info, err := container.Info(ctx)
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("failed to get container info: %w", err)
// 	// }
// 	// snapshotName := info.Snapshotter
// 	// // sn,err:=client.SnapshotService(snapshotter).View(ctx,container.ID(),snapshotName)
// 	// // if err!=nil{
// 	// // 	return nil,fmt.Errorf("failed to get snapshot: %w",err)
// 	// // }
// 	// // defer client.SnapshotService(snapshotName).Remove(ctx,container.ID())

// 	// // commit容器的文件系统到新镜像
// 	// err=client.SnapshotService(snapshotter).Commit(ctx,committedImageName,snapshotName)
// 	// if err!=nil{
// 	// 	return nil,fmt.Errorf("failed to commit container: %w",err)
// 	// }

// 	// // 提交容器为新镜像
// 	// committedImage := client.SnapshotService(snapshotter).Commit(ctx, committedImageName, container.Snapshot(ctx))
// 	// if err != nil {
// 	// 	return nil, fmt.Errorf("failed to commit container: %w", err)
// 	// }

// 	// fmt.Printf("successfully created new image: %s\n", committedImageName)
// 	// return committedImage, nil
// }

// // 删除容器
// func deleteContainer(ctx context.Context, client *containerd.Client, containerName string) error {
// 	fmt.Printf("deleting container: %s ...\n", containerName)

// 	container, err := client.LoadContainer(ctx, containerName)
// 	if err != nil {
// 		if errdefs.IsNotFound(err) {
// 			return nil // 容器不存在，无需删除
// 		}
// 		return fmt.Errorf("failed to load container: %w", err)
// 	}

// 	// 获取任务并停止/删除
// 	task, err := container.Task(ctx, nil)
// 	if err == nil {
// 		// 停止任务
// 		if status, err := task.Status(ctx); err == nil && status.Status == containerd.Running {
// 			if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
// 				return fmt.Errorf("failed to stop container: %w", err)
// 			}
// 			// 等待容器停止
// 			if _, err := task.Wait(ctx); err != nil {
// 				return fmt.Errorf("failed to wait for container to stop: %w", err)
// 			}
// 		}

// 		// 删除任务
// 		if _, err := task.Delete(ctx); err != nil {
// 			return fmt.Errorf("failed to delete container task: %w", err)
// 		}
// 	}

// 	// 删除容器
// 	if err := container.Delete(ctx); err != nil {
// 		return fmt.Errorf("failed to delete container: %w", err)
// 	}

// 	return nil
// }
