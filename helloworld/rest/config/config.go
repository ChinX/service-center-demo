package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-yaml/yaml"
)

// microservice.yaml 配置
type Config struct {
	Service  *MicroService `yaml:"service"`
	Registry *Registry     `yaml:"registry"`
	Provider *MicroService `yaml:"provider"`
	Tenant   *Tenant       `yaml:"tenant"`
}

// 微服务配置
type MicroService struct {
	ID       string    `yaml:"-"`
	AppID    string    `yaml:"appId"`
	Name     string    `yaml:"name"`
	Version  string    `yaml:"version"`
	Instance *Instance `yaml:"instance"`
}

// 实例配置
type Instance struct {
	ID            string `yaml:"-"`
	Hostname      string `yaml:"hostname"`
	Protocol      string `yaml:"protocol"`
	ListenAddress string `yaml:"listenAddress"`
}

// Service-Center 配置
type Registry struct {
	Address   string   `yaml:"address"`
	Endpoints []string `yaml:"-"`
}

// 租户信息
type Tenant struct {
	Domain string `yaml:"domain"`
	Project string `yaml:"project"`
}

// 加载配置
func LoadConfig(filePath string) (*Config, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	conf := &Config{}

	err = yaml.Unmarshal(data, &conf)
	if err != nil {
		return nil, err
	}
	if conf.Service == nil {
		return nil, errors.New("microservice is empty")
	}

	if conf.Tenant == nil {
		conf.Tenant = &Tenant{}
	}

	if conf.Tenant.Domain == "" {
		conf.Tenant.Domain = "default"
	}

	if conf.Tenant.Project == "" {
		conf.Tenant.Project = "default"
	}

	if conf.Service.Instance != nil {
		if conf.Service.Instance.Hostname == "" {
			conf.Service.Instance.Hostname, _ = os.Hostname()
		}

		if conf.Service.Instance.ListenAddress == "" {
			return nil, fmt.Errorf("instance lister address is empty")
		}

		host, port, err := net.SplitHostPort(conf.Service.Instance.ListenAddress)
		if err != nil {
			return nil, fmt.Errorf("instance lister address is wrong: %s", err)
		}
		if host == "" {
			host = "127.0.0.1"
		}
		num, err := strconv.Atoi(port)
		if err != nil || num <= 0 {
			return nil, fmt.Errorf("instance lister port %s is wrong: %s", port, err)
		}
		conf.Service.Instance.ListenAddress = host + ":" + port
	}

	if conf.Registry == nil || conf.Registry.Address == "" {
		return nil, errors.New("registry is empty")
	}

	conf.Registry.Endpoints = strings.Split(conf.Registry.Address, ",")
	for i := 0; i < len(conf.Registry.Endpoints); i++ {
		_, err := url.Parse(conf.Registry.Endpoints[i])
		if err != nil {
			return nil, fmt.Errorf("parse registry address faild: %s", err)
		}
	}
	return conf, nil
}
