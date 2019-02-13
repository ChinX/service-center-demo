package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/apache/servicecomb-service-center/pkg/client/sc"
	"github.com/chinx/service-center-demo/helloworld/rest/config"
	"github.com/chinx/service-center-demo/helloworld/rest/servicecenter"
)

var conf *config.Config

func main() {
	// 加载配置文件
	var err error
	conf, err = config.LoadConfig("./conf/microservice.yaml")
	if err != nil {
		log.Fatalf("load config file faild: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go run(ctx)
	fmt.Println("awaiting system signal")
	awaitingSystemSignal()
	cancel()
	stop()
	fmt.Println("exiting")
}

// 监听系统终止信号
func awaitingSystemSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

	sig := <-sigChan
	fmt.Println("close instance by:", sig)
}

func stop() {
	err := servicecenter.Unregister(context.Background(), conf.Service)
	if err != nil {
		log.Println(err)
	}
}

func run(ctx context.Context) {
	// 启动http监听
	if conf.Service.Instance != nil {
		go httpListener(conf.Service.Instance.ListenAddress)
	}

	// 初始化 service-center
	servicecenter.InitRegistry(conf.Tenant.Domain+"/"+conf.Tenant.Project, conf.Registry)
	serviceID, instanceID, err := servicecenter.Register(ctx, conf.Service)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("ServiceID:", serviceID)
	log.Println("InstanceID:", instanceID)

	conf.Service.ID = serviceID
	if conf.Service.Instance != nil {
		conf.Service.Instance.ID = instanceID
	}

	// 启动心跳
	go servicecenter.Heartbeat(ctx, conf.Service)
	discoverProvider(ctx, serviceID)
	sayHello(ctx)
}

func httpListener(listenAddress string) {
	// 启动 http 监听
	http.HandleFunc("/sayhello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sayHello(r.Context())))
	})
	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Fatalln(err)
	}
}

func discoverProvider(ctx context.Context, serviceID string) {
	if conf.Provider != nil {
		providerID, err := servicecenter.Discovery(ctx, serviceID, conf.Provider)
		if err != nil {
			log.Fatalln(err)
		}
		conf.Provider.ID = providerID
		go servicecenter.WatchProvider(ctx, conf.Service.ID)
	}
}

// 与 provider 通讯
func sayHello(ctx context.Context) string {
	endPoints, err := servicecenter.ProviderEndpoints(conf.Provider)
	if err != nil {
		return fmt.Sprintf("get provider endpoints faild: %s", err)
	}
	client, err := sc.NewLBClient(endPoints, (&sc.Config{Endpoints: endPoints}).Merge())
	if err != nil {
		return fmt.Sprintf("new lb client faild: %s", err)
	}
	resp, err := client.RestDoWithContext(ctx, http.MethodGet, "/hello", nil, nil)
	if err != nil {
		return fmt.Sprintf("do request faild: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("read response faild: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("do request failed, response statusCode: %d, body: %s",
			resp.StatusCode, string(body))
	}
	message := string(body)
	log.Printf("reply form provider: %s", message)
	return message
}
