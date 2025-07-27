package internal

import (
	"context"
	"fmt"

	// "io"
	"log"
	"os"
	"strings"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/nerdctl/v2/pkg/api/types"
	"github.com/containerd/nerdctl/v2/pkg/cmd/container"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	// "github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/oci"
)

type Server struct {
	runtimeServiceClient runtimeapi.RuntimeServiceClient // 与containerd进行交互的client
	containerdClient     *client.Client                  // 也是与containerd进行交互的client
	// imageClient          imageutil.ImageInterface        // 与image进行交互的client
}

const (
	namespace = "test.io"
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
	log.Println("context.Namespace: ", ctx.Value("namespace"))
	log.Println("namespace: ", namespace)
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

	// // start container
	// startContainerReq := &runtimeapi.StartContainerRequest{
	// 	ContainerId: responseContainer.ContainerId,
	// }
	// _, err = s.StartContainer(ctx, startContainerReq)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to start container: %v", err)
	// }

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
		// Stdout:   io.Discard,
		Stdout:   os.Stdout,
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

// CreateContainerDirectly 直接使用 containerd API 创建容器，绕过 CRI
func (s *Server) CreateContainerDirectly(ctx context.Context, containerName, imageName, namespace string) (string, error) {
	// 1. 获取镜像
	image, err := s.containerdClient.GetImage(ctx, imageName)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %v", err)
	}

	// 2. 创建容器
	// 添加 annotations/labels
	annotations := map[string]string{
		"devbox.sealos.io/content-id": "cri-containerd-direct",
		"namespace":                   namespace,
		"image.name":                  imageName,
		// "container.type":              "direct",
		// "description":                 "Container created directly via containerd API",
	}

	container, err := s.containerdClient.NewContainer(ctx, containerName,
		client.WithImage(image),
		client.WithNewSnapshot(containerName, image),
		client.WithContainerLabels(annotations), // 添加 annotations
		client.WithNewSpec(oci.WithImageConfig(image),
			oci.WithProcessArgs("/bin/sh", "-c", "while true; do echo 'Hello, World!'; sleep 5; done"),
			oci.WithHostname("test-container"),
		),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %v", err)
	}

	// // 3. 创建任务：启动容器
	// task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create task: %v", err)
	// }

	// // 4. 启动任务
	// err = task.Start(ctx)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to start task: %v", err)
	// }

	return container.ID(), nil
}

// GetContainerAnnotations 获取容器的 annotations
func (s *Server) GetContainerAnnotations(ctx context.Context, containerName string) (map[string]string, error) {
	container, err := s.containerdClient.LoadContainer(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to load container: %v", err)
	}

	// 获取容器的 labels（即 annotations）
	labels, err := container.Labels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container labels: %v", err)
	}
	return labels, nil
}

// GetContainerInfo 获取容器的详细信息
func (s *Server) GetContainerInfo(ctx context.Context, containerName string) (interface{}, error) {
	container, err := s.containerdClient.LoadContainer(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to load container: %v", err)
	}

	// 获取容器的详细信息
	info, err := container.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container info: %v", err)
	}

	return info, nil
}

// DeleteContainerDirectly 直接删除容器（修复版本）
func (s *Server) DeleteContainerDirectly(ctx context.Context, containerName string) error {
	// 1. 加载容器
	container, err := s.containerdClient.LoadContainer(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to load container: %v", err)
	}

	// 2. 尝试获取并停止任务
	task, err := container.Task(ctx, nil)
	if err == nil {
		log.Printf("Stopping task for container: %s", containerName)

		// 直接强制杀死任务（避免等待问题）
		err = task.Kill(ctx, 9) // SIGKILL
		if err != nil {
			log.Printf("Warning: failed to send SIGKILL: %v", err)
		} else {
			log.Printf("Sent SIGKILL to task")
		}

		// 简短等待后删除任务
		log.Printf("Deleting task...")
		_, err = task.Delete(ctx, client.WithProcessKill)
		if err != nil {
			log.Printf("Warning: failed to delete task: %v", err)
		} else {
			log.Printf("Task deleted for container: %s", containerName)
		}
	}

	// 3. 删除容器（包含快照）
	err = container.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete container: %v", err)
	}

	log.Printf("Container deleted: %s", containerName)
	return nil
}

// ListContainers 列出容器
func (s *Server) ListContainers(ctx context.Context) ([]string, error) {
	containers, err := s.containerdClient.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %v", err)
	}

	var containerIDs []string
	for _, container := range containers {
		containerIDs = append(containerIDs, container.ID())
	}

	return containerIDs, nil
}

// CleanupOrphanContainers 清理孤儿容器（垃圾回收）
func (s *Server) CleanupOrphanContainers(ctx context.Context, namespace string) error {

	log.Printf("Starting GC in namespace: %s", namespace)

	// get all container in namespace
	containers,err:=s.containerdClient.Containers(ctx)
	if err !=nil{
		log.Printf("Failed to get containers, err: %v", err)
		return err
	}

	var deletedContainersCount int
	for _,container:=range containers{
		// if get container's labels failed, skip
		labels,err:=container.Labels(ctx)
		if err!=nil{
			log.Printf("Failed to get labels for container %s, err: %v",container.ID(),err)
			continue
		}
		// if container is not devbox container, skip
		if _,ok:=labels["devbox.sealos.io/content-id"];!ok{
			continue 
		}

		// get container task
		task,err:=container.Task(ctx,nil)
		if err!=nil{
			// delete orphan container
			log.Printf("Found Orphan Container: %s",container.ID())
			err=container.Delete(ctx,client.WithSnapshotCleanup)
			if err!=nil{
				log.Printf("Failed to delete Orphan Container %s, err: %v",container.ID(),err)
			}else{
				log.Printf("Deleted Orphan Container: %s successfully",container.ID())
				deletedContainersCount++
			}
			continue
		}

		status,err:=task.Status(ctx)
		if err!=nil{
			log.Printf("Failed to get task status for container %s, err: %v",container.ID(),err)
			continue
		}
		if status.Status!=client.Running{
			// delete task
			_,err=task.Delete(ctx,client.WithProcessKill)
			if err!=nil{
				log.Printf("Failed to delete task for container %s, err: %v",container.ID(),err)
			}

			// delete container and snapshot
			err=container.Delete(ctx,client.WithSnapshotCleanup)
			if err!=nil{
				log.Printf("Failed to delete container %s, err: %v",container.ID(),err)
			}else{
				log.Printf("Deleted Container: %s successfully",container.ID())
				deletedContainersCount++
			}
		}

	}
	log.Printf("GC completed, deleted %d containers",deletedContainersCount)
	return nil


	// log.Printf("Starting cleanup in namespace: %s", namespace)

	// // 获取所有容器
	// containers, err := s.containerdClient.Containers(ctx)
	// if err != nil {
	// 	return fmt.Errorf("failed to list containers: %v", err)
	// }

	// var deletedCount int
	// for _, container := range containers {
	// 	// 检查容器是否有对应的运行任务
	// 	task, err := container.Task(ctx, nil)
	// 	if err != nil {
	// 		// 没有任务的容器可能是孤儿容器
	// 		log.Printf("Found orphan container (no task): %s", container.ID())

	// 		// 尝试删除
	// 		err = container.Delete(ctx, client.WithSnapshotCleanup)
	// 		if err != nil {
	// 			log.Printf("Failed to delete orphan container %s: %v", container.ID(), err)
	// 		} else {
	// 			log.Printf("Deleted orphan container: %s", container.ID())
	// 			deletedCount++
	// 		}
	// 		continue
	// 	}

	// 	// 检查任务状态
	// 	status, err := task.Status(ctx)
	// 	if err != nil {
	// 		log.Printf("Failed to get task status for %s: %v", container.ID(), err)
	// 		continue
	// 	}

	// 	// 清理已停止的容器
	// 	if status.Status != "running" {
	// 		log.Printf("Found stopped container: %s (status: %s)", container.ID(), status.Status)

	// 		// 删除任务
	// 		_, err = task.Delete(ctx)
	// 		if err != nil {
	// 			log.Printf("Failed to delete task for %s: %v", container.ID(), err)
	// 		}

	// 		// 删除容器
	// 		err = container.Delete(ctx, client.WithSnapshotCleanup)
	// 		if err != nil {
	// 			log.Printf("Failed to delete stopped container %s: %v", container.ID(), err)
	// 		} else {
	// 			log.Printf("Deleted stopped container: %s", container.ID())
	// 			deletedCount++
	// 		}
	// 	}
	// }

	// log.Printf("Cleanup completed. Deleted %d containers", deletedCount)
	// return nil
}
