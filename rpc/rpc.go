package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type (
	Client struct {
		address url.URL
	}

	rpcCall struct {
		Method string        `json:"method"`
		ID     *string       `json:"id"`
		Params []interface{} `json:"params"`
	}

	rpcResult struct {
		Result *json.RawMessage       `json:"result"`
		Error  map[string]interface{} `json:"error"`
		ID     *string                `json:"id"`
	}

	rpcResultError map[string]interface{}
)

func (rpe rpcResultError) Error() string {
	return fmt.Sprint(map[string]interface{}(rpe))
}

func NewClient(address string) (*Client, error) {
	url, err := url.Parse(address)
	if err != nil {
		return nil, err
	}

	return &Client{
		address: *url,
	}, nil
}

func (c *Client) Call(method string, params []interface{}, result interface{}) error {
	data, err := json.Marshal(rpcCall{
		Method: method,
		ID:     nil,
		Params: params,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(c.address.String(), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResult rpcResult
	if err := json.NewDecoder(resp.Body).Decode(&rpcResult); err != nil {
		return err
	}

	if rpcResult.Error != nil {
		return rpcResultError(rpcResult.Error)
	}

	if rpcResult.Result != nil {
		if err := json.Unmarshal([]byte(*rpcResult.Result), result); err != nil {
			return err
		}
	}

	return nil
}
