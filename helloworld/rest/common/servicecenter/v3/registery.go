package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/apache/servicecomb-service-center/server/core/proto"
	"github.com/chinx/service-center-demo/helloworld/rest/common/restful"
	"github.com/gorilla/websocket"
)

var (
	// 接口 API 定义
	apiExistenceURL           = "/registry/v3/existence"
	apiMicroServicesURL       = "/registry/v3/microservices"
	apiMicroServiceURL        = "/registry/v3/microservices/%s"
	apiMicroServiceWatcherURL = "/registry/v3/microservices/%s/watcher"
	apiServiceInstancesURL    = "/registry/v3/microservices/%s/instances"
	apiServiceInstanceURL     = "/registry/v3/microservices/%s/instances/%s"
	apiDiscoveryInstancesURL  = "/registry/v3/instances"
	apiHeartbeatsURL          = "/registry/v3/heartbeats"

	microServiceType sourceType = "microservice"
	schemaType       sourceType = "schema"
)

type sourceType string

type Client struct {
	domain string
	*restful.Client
}

func NewClient(domain string, endpoints ...string) *Client {
	return &Client{domain: domain, Client: restful.NewClient(endpoints...)}
}

// 查询微服务是否存在
func (c *Client) existence(val url.Values) (*proto.GetExistenceResponse, error) {
	reqAPI := apiExistenceURL + "?" + val.Encode()
	resp, err := c.Do(http.MethodGet, reqAPI, c.DefaultHeaders(), nil)
	if err == nil {
		respData := &proto.GetExistenceResponse{}
		err = c.ParseResponse(resp, http.StatusOK, respData)
		if err == nil {
			return respData, nil
		}
	}
	return nil, err
}

// 获取微服务服务ID
func (c *Client) GetServiceID(service *proto.MicroService) (string, error) {
	val := url.Values{}
	val.Set("type", string(microServiceType))
	val.Set("appId", service.AppId)
	val.Set("serviceName", service.ServiceName)
	val.Set("version", service.Version)
	respData, err := c.existence(val)
	if err == nil {
		return respData.ServiceId, nil
	}
	return "", fmt.Errorf("[GetServiceID]: %s", err)
}

// 注册微服务
func (c *Client) RegisterService(service *proto.MicroService) (string, error) {
	resp, err := c.Do(http.MethodPost, apiMicroServicesURL,
		c.DefaultHeaders(), &proto.CreateServiceRequest{Service: service})
	if err == nil {
		result := &proto.CreateServiceResponse{}
		err = c.ParseResponse(resp, http.StatusOK, result)
		if err == nil {
			return result.ServiceId, nil
		}
	}
	return "", fmt.Errorf("[RegisterService]: %s", err)
}

// 注销微服务
func (c *Client) UnregisterService(service *proto.MicroService) error {
	reqAPI := fmt.Sprintf(apiMicroServiceURL, service.ServiceId)
	resp, err := c.Do(http.MethodDelete, reqAPI, c.DefaultHeaders(), nil)
	if err == nil {
		err = c.ParseResponse(resp, http.StatusOK, nil)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("[UNRegisterService]: %s", err)
}

// 服务发现
func (c *Client) Discovery(consumerId string, provider *proto.MicroService) ([]*proto.MicroServiceInstance, error) {
	val := url.Values{}
	val.Set("appId", provider.AppId)
	val.Set("serviceName", provider.ServiceName)
	val.Set("version", provider.Version)

	handler := c.DefaultHeaders()
	handler.Set("x-consumerid", consumerId)
	resp, err := c.Do(http.MethodGet, apiDiscoveryInstancesURL+"?"+val.Encode(),
		handler, nil)
	if err == nil {

		respData := &proto.GetInstancesResponse{}
		err = c.ParseResponse(resp, http.StatusOK, respData)
		if err == nil {
			return respData.Instances, nil
		}
	}
	return nil, fmt.Errorf("[Discovery]: %s", err)
}

// 服务订阅
func (c *Client) WatchService(ctx context.Context, serviceID string, callback func(*proto.WatchInstanceResponse)) error {
	conn, err := c.WebsocketDial(fmt.Sprintf(apiMicroServiceWatcherURL, serviceID), c.DefaultHeaders())
	if err != nil {
		return fmt.Errorf("[WatchService]: start websocket faild: %s", err)
	}

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}
		if messageType == websocket.TextMessage {
			data := &proto.WatchInstanceResponse{}
			err := json.Unmarshal(message, data)
			if err != nil {
				log.Println(err)
				break
			}
			callback(data)
		}
	}
	return fmt.Errorf("[WatchService]: receive message faild: %s", err)
}

// 注册微服务实例
func (c *Client) RegisterInstance(serviceID string, instance *proto.MicroServiceInstance) (string, error) {
	reqData := &proto.RegisterInstanceRequest{Instance: instance}

	reqAPI := fmt.Sprintf(apiServiceInstancesURL, serviceID)
	resp, err := c.Do(http.MethodPost, reqAPI, c.DefaultHeaders(), reqData)
	if err == nil {
		result := &proto.RegisterInstanceResponse{}
		err = c.ParseResponse(resp, http.StatusOK, result)
		if err == nil {
			return result.InstanceId, nil
		}
	}
	return "", fmt.Errorf("[RegisterInstance]: %s", err)
}

// 注销微服务实例
func (c *Client) UnregisterInstance(serviceID string, instance *proto.MicroServiceInstance) error {
	reqAPI := fmt.Sprintf(apiServiceInstanceURL, serviceID, instance.InstanceId)
	resp, err := c.Do(http.MethodDelete, reqAPI, c.DefaultHeaders(), nil)
	if err == nil {
		err = c.ParseResponse(resp, http.StatusOK, nil)
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("[UNRegisterInstance]: %s", err)
}

// 心跳保活
func (c *Client) Heartbeat(instances ...*proto.HeartbeatSetElement) error {
	reqData := &proto.HeartbeatSetRequest{Instances: instances}

	reqAPI := apiHeartbeatsURL
	resp, err := c.Do(http.MethodPut, reqAPI, c.DefaultHeaders(), reqData)
	if err == nil {
		err = c.ParseResponse(resp, http.StatusOK, nil)
	}
	if err != nil {
		return fmt.Errorf("[Heartbeat]: %s", err)
	}
	return nil
}

// 设置默认头部
func (c *Client) DefaultHeaders() http.Header {
	headers := http.Header{
		"Content-Type":  []string{"application/json"},
		"X-Domain-Name": []string{"default"},
	}
	if c.domain != "" {
		headers.Set("X-Domain-Name", c.domain)
	}
	return headers
}
