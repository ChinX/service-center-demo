package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
}

func httpListener(listenAddress string) {
	// 启动 http 监听
	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		log.Println("request hello")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	})
	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Fatalln(err)
	}
}
