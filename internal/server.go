package internal

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/nerdctl/v2/pkg/api/types"
	"github.com/containerd/nerdctl/v2/pkg/cmd/container"
	"github.com/google/uuid"

	// imageutil "github.com/labring/cri-shim/pkg/image"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Server struct {
	runtimeServiceClient runtimeapi.RuntimeServiceClient // 与containerd进行交互的client
	containerdClient     *client.Client                  // 也是与containerd进行交互的client
	// imageClient          imageutil.ImageInterface        // 与image进行交互的client
}

const (
	namespace = "k8s.io"
	address   = "unix:///var/run/containerd/containerd.sock"
	// fStdout = os.Stdout // 或 os.DevNull
)

// 获取Server
func GetServer() (*Server, error) {
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
	// fStdout := os.Stdout // 或 os.DevNull

	// imageClient, err := imageutil.NewImageInterface(namespace, address, fStdout)
	// if err != nil {
	// 	panic(err)
	// }

	return &Server{
		runtimeServiceClient: client,
		containerdClient:     containerdClient,
		// imageClient:          imageClient,
	}, nil
}

// 拉取镜像：拉取镜像本质还是使用的containerd.Client.Pull()
func (s *Server) PullImage(ctx context.Context, imageName string) (client.Image, error) {
	log.Default().Printf("pulling image: %s ...\n", imageName)
	image, err := s.containerdClient.Pull(ctx, imageName, client.WithPullUnpack)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %s, err: %v", imageName, err)
	}
	log.Default().Printf("pulled image: %s successfully! \n", image.Name())
	return image, nil
}

// 创建PodSandbox: 创建PodSandbox本质是使用的runtimeServiceClient.RunPodSandbox()
func (s *Server) RunPodSandbox(ctx context.Context, request *runtimeapi.RunPodSandboxRequest) (*runtimeapi.RunPodSandboxResponse, error) {
	log.Default().Println("Doing run pod sandbox request", "request", request)
	return s.runtimeServiceClient.RunPodSandbox(ctx, request)
}

// 创建容器: 调用了runPodSandbox
func (s *Server) CreateContainer(ctx context.Context, containerName string, image client.Image, namespace string) (*runtimeapi.CreateContainerResponse, error) {
	// 先创建PodSandbox
	// podName := "test-pod"
	cgroupParent := "system.slice"

	// 生成唯一且安全的 Pod 名称
	podUUID := uuid.New().String()
	podName := fmt.Sprintf("test-pod-%s", strings.ReplaceAll(podUUID, "-", "")) // 移除短横线
	// contentID := fmt.Sprintf("content-%s", podUUID)

	podSandboxReq := &runtimeapi.RunPodSandboxRequest{
		Config: &runtimeapi.PodSandboxConfig{
			Metadata: &runtimeapi.PodSandboxMetadata{
				Name:      podName,
				Namespace: namespace,
				Uid:       podUUID,
				Attempt:   1,
			},
			Hostname:     "my-pod",
			LogDirectory: "/var/log/pods/my-pod",
			Labels: map[string]string{
				"app": "my-app",
			},
			Annotations: map[string]string{
				// "devbox.sealos.io/content-id": contentID,
				"description": "my-pod-description",
			},
			Linux: &runtimeapi.LinuxPodSandboxConfig{
				CgroupParent: cgroupParent, // 格式为 slice:prefix:name
			},
		},
	}

	response, err := s.RunPodSandbox(ctx, podSandboxReq)
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

	// start container
	startContainerReq := &runtimeapi.StartContainerRequest{
		ContainerId: responseContainer.ContainerId,
	}
	_, err = s.StartContainer(ctx, startContainerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %v", err)
	}

	return responseContainer, nil
}

// 运行容器
func (s *Server) StartContainer(ctx context.Context, request *runtimeapi.StartContainerRequest) (*runtimeapi.StartContainerResponse, error) {
	log.Default().Println("Doing start container request", "request", request)
	return s.runtimeServiceClient.StartContainer(ctx, request)
}

// commit容器
func (s *Server) CommitContainer(ctx context.Context, containerID, committedImageName string) error {
	global := types.GlobalCommandOptions{
		Namespace:        namespace,
		Address:          address,
		DataRoot:         "/var/lib/containerd",
		InsecureRegistry: true,
	}
	opt := types.ContainerCommitOptions{
		Stdout:   io.Discard,
		GOptions: global,
		Pause:    false,
		DevboxOptions: types.DevboxOptions{
			RemoveBaseImageTopLayer: true,
		},
	}
	return container.Commit(ctx, s.containerdClient, committedImageName, containerID, opt)
}

// delete容器
func (s *Server) DeleteContainer(ctx context.Context, containerID string) error {
	req := &runtimeapi.RemoveContainerRequest{
		ContainerId: containerID,
	}
	_, err := s.runtimeServiceClient.RemoveContainer(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete container: %v", err)
	}
	return nil
}
