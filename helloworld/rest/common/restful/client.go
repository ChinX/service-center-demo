package restful

import (
	"log"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type Client struct {
	LoadBalancer
	retries int
}

type LoadBalancer interface {
	Next() string
}

type RoundRobinLB struct {
	Endpoints []string
	index     int32
}

func NewLoadBalancer(endpoints []string) LoadBalancer {
	lb := &RoundRobinLB{
		Endpoints: make([]string, len(endpoints)),
		index:     -1,
	}
	copy(lb.Endpoints, endpoints)
	return lb
}

func (lb *RoundRobinLB) Next() string {
	l := len(lb.Endpoints)
	if l == 0 {
		return ""
	}
	c := atomic.LoadInt32(&lb.index)
	if c >= int32(l)-1 {
		atomic.StoreInt32(&lb.index, 0)
		return lb.Endpoints[0]
	} else if atomic.CompareAndSwapInt32(&lb.index, c, c+1) {
		return lb.Endpoints[c+1]
	}
	return lb.Endpoints[atomic.LoadInt32(&lb.index)]
}

func NewClient(endpoints ...string) *Client {
	return &Client{LoadBalancer: NewLoadBalancer(endpoints), retries: len(endpoints)}
}

func (c *Client) Do(method string, api string, headers http.Header, body interface{}) (resp *http.Response, err error) {
	for i := 0; i < c.retries; i++ {
		address := c.Next() + api
		resp, err = DoRequest(method, address, headers, body)
		if err != nil {
			log.Printf("%s request to %s faild: %s", method, address, err)
			continue
		}
		break
	}
	return
}

func (c *Client) ParseResponse(resp *http.Response, expectCode int, expectData interface{}) error {
	return ParseResponse(resp, expectCode, expectData)
}

func (c *Client) WebsocketDial(api string, headers http.Header) (conn *websocket.Conn, err error) {
	for i := 0; i < c.retries; i++ {
		address := c.Next() + api
		addr, err := url.Parse(address)
		if err != nil {
			log.Printf("parse websocket dial url %s faild: %s", address, err)
			continue
		}
		if addr.Scheme == "https" {
			addr.Scheme = "wss"
		} else {
			addr.Scheme = "ws"
		}
		conn, _, err = (&websocket.Dialer{}).Dial(addr.String(), headers)
		if err != nil {
			log.Printf("websocket dial to %s faild: %s", address, err)
			continue
		}
		break
	}
	return
}
