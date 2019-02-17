package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/apache/servicecomb-service-center/pkg/client/sc"
	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/chinx/service-center-demo/helloworld/rest/config"
)

var conf *config.Config

func main() {
	// 加载配置文件
	var err error
	conf, err = config.LoadConfig("./conf/microservice.yaml")
	if err != nil {
		log.Fatalf("load config file faild: %s", err)
	}

	// 从服务中心发现服务端实例，并与之通讯
	endpoints, err := discoverProvider()
	if err != nil {
		log.Fatalf("discover and requset faild: %s", err)
	}

	// 与 provider 服务通讯
	err = sayHello(endpoints)
	if err != nil {
		log.Fatalf("say hello to provider faild: %s", err)
	}
}

func discoverProvider() ([]string, error) {
	ctx := context.Background()

	// 创建 sc client
	client, err := sc.NewSCClient(sc.Config{Endpoints: conf.Registry.Endpoints})
	if err != nil {
		return nil, fmt.Errorf("new sc client faild: %s", err)
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
			return nil, fmt.Errorf("create service faild: %s", err)
		}
		serviceID = nid
	}
	log.Println("[create consumer] serviceId:", serviceID)

	if conf.Provider == nil {
		return nil, fmt.Errorf("provider config not found")
	}

	list, err1 := client.DiscoveryInstances(ctx, domainProject, serviceID,
		conf.Provider.AppID, conf.Provider.Name, conf.Provider.VersionRule)
	if err1 != nil || len(list) == 0 {
		return nil, fmt.Errorf("provider not found, serviceName: %s appID: %s, versionRule: %s",
			conf.Provider.Name, conf.Provider.AppID, conf.Provider.VersionRule)
	}

	endpoints := make([]string, 0, len(list))

	// 解析实例 endpoint 为普通 http 地址
	for i := 0; i < len(list); i++ {
		es := list[i].Endpoints
		for j := 0; j < len(es); j++ {
			addr, err := url.Parse(es[j])
			if err != nil {
				log.Printf("parse provider endpoint faild: %s", err)
				continue
			}
			if addr.Scheme == "rest" {
				addr.Scheme = "http"
			}
			endpoints = append(endpoints, addr.String())
		}
	}
	log.Println("[discovery provider instances] endpoints:", endpoints)
	return endpoints, nil
}

// 与 provider 通讯
func sayHello(endpoints []string) error {
	// 创建负载均衡 client
	client, err := sc.NewLBClient(endpoints, (&sc.Config{Endpoints: endpoints}).Merge())
	if err != nil {
		return fmt.Errorf("new lb client faild: %s", err)
	}

	log.Println("send request to provider")

	// 发送http请求
	resp, err := client.RestDoWithContext(context.Background(), http.MethodGet, "/hello", nil, nil)
	if err != nil {
		return fmt.Errorf("do request faild: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response faild: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("do request failed, response statusCode: %d, body: %s",
			resp.StatusCode, string(body))
	}
	message := string(body)
	log.Printf("reply form provider: %s", message)
	return nil
}
