package servicecenter

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/apache/servicecomb-service-center/pkg/client/sc"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/chinx/service-center-demo/helloworld/rest/config"
)

var (
	domainProject     string
	cli               *sc.SCClient
	heartbeatInterval  = 30 * time.Second
	providerCaches        = &sync.Map{}
)

func InitRegistry(tenant string, registry *config.Registry) (err error) {
	cli, err = sc.NewSCClient(sc.Config{Endpoints: registry.Endpoints})
	if err == nil {
		domainProject = tenant
	}
	return err
}

func Register(ctx context.Context, svc *config.MicroService) (string, string, error) {
	service := transformMicroService(svc)

	// 检测微服务是否存在
	serviceID, err := cli.ServiceExistence(ctx, domainProject, sc.MicroServiceType, service.AppId, service.ServiceName, service.Version, svc)
	if serviceID == "" {
		// 注册微服务
		serviceID, err = cli.CreateService(ctx, domainProject, service)
		if err != nil {
			return "", "", err
		}
	}

	if svc.Instance == nil {
		return serviceID, "", nil
	}

	// 注册微服务实例
	instance := transformInstance(svc.Instance)
	instanceID, err := cli.RegisterInstance(ctx, domainProject, serviceID, instance)
	if err != nil{
		return "", "", err
	}
	return serviceID, instanceID, nil
}

func Unregister(ctx context.Context, svc *config.MicroService) error {
	if svc.Instance != nil {
		// 注销微服务实例
		err := cli.UnregisterInstance(ctx, domainProject, svc.ID, svc.Instance.ID)
		if err != nil {
			return err
		}
	}

	// 实例注销后，服务中心清理数据需要一些时间，稍作延后
	time.Sleep(time.Second * 3)
	// 注销微服务
	return cli.DeleteService(ctx, domainProject, svc.ID)
}

func Discovery(ctx context.Context, consumerId string, provider *config.MicroService) (string, error) {
	service := transformMicroService(provider)
	list, err := cli.DiscoveryInstances(ctx, domainProject, consumerId, service)
	if err != nil || len(list) == 0 {
		return "", fmt.Errorf("provider not found, serviceName: %s appID: %s, version: %s",
			provider.Name, provider.AppID, provider.Version)
	}
	// 缓存 provider 实例信息
	providerCaches.Store(list[0].ServiceId, list)
	return list[0].ServiceId, nil
}

func ProviderEndpoints(provider *config.MicroService) ([]string, error) {
	list, ok := providerCaches.Load(provider.ID)
	if !ok {
		return nil, fmt.Errorf("provider \"%s\" not found", provider.Name)
	}
	providerList := list.([]*proto.MicroServiceInstance)
	endpointList := make([]string, 0, len(providerList))
	for i := 0; i < len(providerList); i++ {
		endpoints := providerList[i].Endpoints
		for j := 0; j < len(endpoints); j++ {
			addr, err := url.Parse(endpoints[j])
			if err != nil {
				log.Printf("parse provider endpoint faild: %s", err)
				continue
			}
			if addr.Scheme == "rest" {
				addr.Scheme = "http"
			}
			endpointList = append(endpointList, addr.String())
		}
	}
	return endpointList, nil
}

// 订阅服务，更新 provider 缓存
func WatchProvider(ctx context.Context, serviceID string) {
	err := cli.Watch(ctx, domainProject, serviceID, func(result *proto.WatchInstanceResponse) {
		log.Println("reply from watch service")
		list, ok := providerCaches.Load(result.Instance.ServiceId)
		if !ok {
			log.Printf("provider \"%s\" not found", result.Instance.ServiceId)
			return
		}
		providerList := list.([]*proto.MicroServiceInstance)

		renew := false
		for i, l := 0, len(providerList); i < l; i++ {
			if providerList[i].InstanceId == result.Instance.InstanceId {
				if result.Action == "DELETE" {
					if i < l-1 {
						providerList = append(providerList[:i], providerList[i+1:]...)
					} else {
						providerList = providerList[:i]
					}
				} else {
					providerList[i] = result.Instance
				}
				renew = true
				break
			}
		}
		if !renew && result.Action != "DELETE" {
			providerList = append(providerList, result.Instance)
		}
		log.Println("update provider list:", providerList)
		providerCaches.Store(result.Instance.ServiceId, providerList)
	})
	if err != nil {
		log.Println(err)
	}
}

func Heartbeat(ctx context.Context, svc *config.MicroService) {
	// 启动定时器：间隔30s
	tk := time.NewTicker(heartbeatInterval)
	for {
		select {
		case <-tk.C:
			// 定时发送心跳
			err := cli.Heartbeat(ctx, domainProject, svc.ID, svc.Instance.ID)
			if err != nil {
				log.Println(err)
				tk.Stop()
				return
			}
			log.Println("send heartbeat success")
		// 监听程序退出
		case <-ctx.Done():
			tk.Stop()
			log.Println("heartbeat is done")
			return
		}
	}
}

func transformMicroService(service *config.MicroService) *proto.MicroService {
	return &proto.MicroService{
		AppId:       service.AppID,
		ServiceId:   service.ID,
		ServiceName: service.Name,
		Version:     service.Version,
	}
}

func transformInstance(instance *config.Instance) *proto.MicroServiceInstance {
	return &proto.MicroServiceInstance{
		InstanceId: instance.ID,
		HostName:   instance.Hostname,
		Endpoints:  []string{instance.Protocol + "://" + instance.ListenAddress},
	}
}
