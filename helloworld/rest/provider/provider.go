package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/apache/servicecomb-service-center/pkg/client/sc"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/chinx/service-center-demo/helloworld/rest/config"
)

var (
	conf              *config.Config
	heartbeatInterval = 30 * time.Second
)

func main() {
	// 加载配置文件
	var err error
	conf, err = config.LoadConfig("./conf/microservice.yaml")
	if err != nil {
		log.Fatalf("load config file faild: %s", err)
	}

	// 启动http监听
	if conf.Service.Instance != nil {
		go httpListener(conf.Service.Instance.ListenAddress)
	}

	// 向服务中心注册
	err = registryToServiceCenter()
	if err != nil {
		log.Fatalf("registry to servicecenter faild: %s", err)
	}
}

func httpListener(listenAddress string) {
	// 启动 http 监听
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		log.Println("request from consumer")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	})
	log.Println("[provider listener]", listenAddress)
	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

func registryToServiceCenter() error {
	ctx := context.Background()

	// 创建 sc client
	client, err := sc.NewSCClient(sc.Config{Endpoints: conf.Registry.Endpoints})
	if err != nil {
		return fmt.Errorf("new sc client faild: %s", err)
	}

	domainProject := conf.Tenant.Domain + "/" + conf.Tenant.Project

	// 检测微服务是否存在
	serviceID, _ := client.ServiceExistence(ctx, domainProject,
		conf.Service.AppID, conf.Service.Name, conf.Service.Version, "")
	if serviceID == "" {

		// 创建微服务信息
		nid, err := client.CreateService(ctx, domainProject, &proto.MicroService{
			AppId:       conf.Service.AppID,
			ServiceId:   conf.Service.ID,
			ServiceName: conf.Service.Name,
			Version:     conf.Service.Version,
		})
		if err != nil {
			return fmt.Errorf("create service faild: %s", err)
		}
		serviceID = nid
	}

	log.Println("[create provider] serviceId:", serviceID)

	if conf.Service.Instance == nil {
		return nil
	}

	// 注册微服务实例
	instanceID, err1 := client.RegisterInstance(ctx, domainProject, serviceID,
		&proto.MicroServiceInstance{
			InstanceId: conf.Service.Instance.ID,
			HostName:   conf.Service.Instance.Hostname,
			Endpoints: []string{
				conf.Service.Instance.Protocol + "://" + conf.Service.Instance.ListenAddress,
			},
		})
	if err1 != nil {
		return fmt.Errorf("register instance faild: %s", err1)
	}
	log.Println("[register provider] instanceId:", instanceID)

	// 定时发送心跳，维持实例 UP 状态
	tk := time.NewTicker(heartbeatInterval) // 启动定时器：间隔30s
	for {
		select {
		case <-tk.C:
			// 定时发送心跳
			err := client.Heartbeat(ctx, domainProject, serviceID, instanceID)
			if err != nil {
				tk.Stop()
				return fmt.Errorf("heartbeat faild: %s", err)
			}
			log.Println("send heartbeat success")
		}
	}
}
