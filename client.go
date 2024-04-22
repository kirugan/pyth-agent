package pyth

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
	"github.com/ybbus/jsonrpc"
	"golang.org/x/net/websocket"
	"sync"
	"sync/atomic"
	"time"
)

const readBufferSize = 5 * 1024 * 1024 // 5 MB

type responseCh chan *jsonrpc.RPCResponse
type SignalCh chan struct{}

type Client struct {
	ws *websocket.Conn

	id          uint64
	responsesMx sync.Mutex
	responses   map[int]responseCh

	subscriptionsMx sync.Mutex
	subscriptions   map[int]SignalCh

	log logrus.FieldLogger
}

func NewClient(url string, log *logrus.Logger) (*Client, error) {
	ws, err := websocket.Dial(url, "", "http://example.org")
	if err != nil {
		return nil, err
	}

	p := &Client{
		ws:            ws,
		responses:     make(map[int]responseCh, 10),
		subscriptions: make(map[int]SignalCh, 10),
		log:           log,
		// id is zero by default and it's ok
	}
	go p.readingLoop()

	return p, nil
}

func (p *Client) restart() {
	p.log.Info("Restarting pyth connection procedure")

	// 1. first clear all in-flight requests
	p.subscriptionsMx.Lock()
	for _, ch := range p.subscriptions {
		// enough to close a channel, inside sendRequestWaitResponse there is check for that
		// caller will remove subscription itself
		close(ch)
	}
	p.subscriptionsMx.Unlock()

	// 2. starting reconnect
	const maxSleepTime = time.Minute
	sleepTime := time.Second
	for {
		ws, err := websocket.DialConfig(p.ws.Config())
		if err != nil {
			p.log.Errorf("Failed to dial websocket in restart(): %v", err)

			// wait some time before next try
			time.Sleep(sleepTime)
			if sleepTime < maxSleepTime {
				// every time increase timeout twice until maxSleepTime is reached
				sleepTime = 2 * sleepTime
			}
		}

		// we don't use additional mutex for ws
		p.ws = ws
		break
	}

	// 3. restarting reading loop again
	go p.readingLoop()
}

func (p *Client) readingLoop() {
	buff := make([]byte, readBufferSize)
	var writePos int

	for {
		// a message might not fit in one frame, therefore we read data frame by frame
		// until there is a failure in json.Unmarshal. Position where we should write storing in writePos var
		nR, err := p.ws.Read(buff[writePos:])
		if err != nil {
			p.log.Errorf("Failed to read from websocket to pyth: %v", err)
			p.restart()
			return
		}

		response := new(jsonrpc.RPCResponse)
		// during decoding reading all messages that were collected inside the buffer starting from 0 till writePos
		// + how many bytes were written
		unmarshalBuff := buff[:writePos+nR]
		err = json.Unmarshal(unmarshalBuff, response)
		if err != nil {
			syntaxErr := new(json.SyntaxError)
			if errors.As(err, &syntaxErr) {
				p.log.Infof(
					"Failed unmarshal jsonrpc response: %v (offset=%d, writePos=%d, nR=%d). Response is too big?",
					syntaxErr.Error(),
					syntaxErr.Offset,
					writePos,
					nR,
				)

				writePos += nR
				continue
			}
			// smth. else - exit
			p.log.Warningf("Unmarshaling error that caused exiting from readingLoop: %v", err)
			break
		}
		// if we succeeded in reading json then reset write position to zero
		writePos = 0

		if response.ID == 0 {
			if response.Error != nil {
				p.log.Warning("Received response error for unknown request ", response.Error)
			}

			var notification SubscriptionNotification
			err = json.Unmarshal(unmarshalBuff, &notification)
			if err != nil {
				p.log.Info("Failed to decode response as subscription notification")
			} else {
				p.notify(notification.Params.Subscription)
			}

			continue
		}

		p.responsesMx.Lock()
		if ch, ok := p.responses[response.ID]; ok {
			ch <- response
		} else {
			p.log.Warningf("Can't find response.ID %d in responses map", response.ID)
		}
		p.responsesMx.Unlock()
	}
}

func (p *Client) notify(subscription int) {
	p.subscriptionsMx.Lock()
	defer p.subscriptionsMx.Unlock()

	if ch, ok := p.subscriptions[subscription]; ok {
		ch <- struct{}{}
	}
}

func (p *Client) requestID() int {
	return int(atomic.AddUint64(&p.id, 1))
}

func (p *Client) registerResponse(requestID int) responseCh {
	ch := make(responseCh, 0)

	p.responsesMx.Lock()
	p.responses[requestID] = ch
	p.responsesMx.Unlock()

	return ch
}

func (p *Client) unregisterResponse(requestID int) {
	p.responsesMx.Lock()
	delete(p.responses, requestID)
	p.responsesMx.Unlock()
}

func (p *Client) sendRequest(request *jsonrpc.RPCRequest) error {
	buff, err := json.Marshal(request)
	if err != nil {
		return err
	}

	_, err = p.ws.Write(buff)
	return err
}

func (p *Client) sendRequestWaitResponse(request *jsonrpc.RPCRequest) (*jsonrpc.RPCResponse, error) {
	ch := p.registerResponse(request.ID)
	defer p.unregisterResponse(request.ID)

	err := p.sendRequest(request)
	if err != nil {
		return nil, err
	}

	response, ok := <-ch
	if !ok {
		// этот запрос попал под shutdown - ответа от сервера не будет
		return nil, fmt.Errorf("closed subscription")
	}

	// additional check in case there is an error in response handling
	if response.ID != request.ID {
		return nil, fmt.Errorf("assert failure: response.ID (%d) != requestID (%d)", response.ID, request.ID)
	}

	if response.Error != nil {
		return nil, response.Error
	}

	return response, nil
}

func (p *Client) GetAllProducts() ([]FullProduct, error) {
	request := jsonrpc.NewRequest("get_all_products")
	request.ID = p.requestID()

	response, err := p.sendRequestWaitResponse(request)
	if err != nil {
		return nil, err
	}

	var products []FullProduct
	err = mapstructure.Decode(response.Result, &products)
	if err != nil {
		return nil, err
	}

	return products, nil
}

func (p *Client) GetProductList() ([]Product, error) {
	request := jsonrpc.NewRequest("get_product_list")
	request.ID = p.requestID()

	response, err := p.sendRequestWaitResponse(request)
	if err != nil {
		return nil, err
	}

	var products []Product
	err = mapstructure.Decode(response.Result, &products)
	if err != nil {
		return nil, err
	}

	return products, nil
}

func (p *Client) SubscribePriceSched(account string) (SignalCh, error) {
	var params = struct {
		Account string `json:"account"`
	}{
		Account: account,
	}

	request := jsonrpc.NewRequest("subscribe_price_sched", params)
	request.ID = p.requestID()

	response, err := p.sendRequestWaitResponse(request)
	if err != nil {
		return nil, err
	}

	var subscriptionResult SubscriptionParams
	err = mapstructure.Decode(response.Result, &subscriptionResult)
	if err != nil {
		return nil, err
	}
	p.log.Infof("Subscription to %q were successful", account)

	ch := make(SignalCh)
	p.subscriptionsMx.Lock()
	p.subscriptions[subscriptionResult.Subscription] = ch
	p.subscriptionsMx.Unlock()

	return ch, nil
}

func (p *Client) UpdatePrice(
	account string,
	price int,
	confidenceInterval int,
	status Status,
) error {
	var params = struct {
		Account string `json:"account"`
		Price   int    `json:"price"`
		Conf    int    `json:"conf"`
		Status  Status `json:"status"`
	}{
		account,
		price,
		confidenceInterval,
		status,
	}

	request := jsonrpc.NewRequest("update_price", params)
	request.ID = p.requestID()

	response, err := p.sendRequestWaitResponse(request)
	if err != nil {
		return err
	}

	if result, ok := response.Result.(float64); !ok || result != 0 {
		p.log.Infof("Result for requestID %d is non zero (%v)", response.ID, response.Result)
	}
	return nil
}
