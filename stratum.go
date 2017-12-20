package main

import (
	"encoding/json"
	"errors"
	"github.com/hashicorp/errwrap"
	"log"
	"math/big"
)

type (
	RequestType  string
	ResponseType string

	Uint128 [16]uint8
	Uint256 [32]uint8

	Request interface {
		Type() RequestType
	}
	Response interface {
		Type() ResponseType
	}
)

type (
	RawRPC struct {
		ID     interface{}      `json:"id"`
		Method RequestType      `json:"method"`
		Params *json.RawMessage `json:"params"`
	}

	RequestBase struct {
		ID     interface{} `json:"id"`
		Method RequestType `json:"method"`
	}

	RequestSubscribe struct {
		RequestBase

		Params []string
	}

	RequestAuthorize struct {
		RequestBase

		Username string
		Password string
	}
	// worker, job, extraNonce2, ntime, nonce
	RequestSubmit struct {
		RequestBase

		Worker string
		Job    string
		NTime  uint32

		NoncePart2 Uint128
		Solution   []byte
	}

	ResponseSubscribeReply struct {
		ID         interface{}
		Session    string
		NoncePart1 Uint128
	}
	// job, prevhash, coinbase1, coinbase2, merkle, blockversion, nbit, ntime, clean
	ResponseNotify struct {
		Job            string
		Version        uint32
		HashPrevBlock  Uint256
		HashMerkleRoot Uint256
		HashReserved   Uint256
		NTime          uint32
		NBits          uint32
		CleanJobs      bool
	}

	ResponseSetDifficulty struct {
		difficulty Uint256
	}

	ResponseGeneral struct {
		ID interface{}

		Result interface{}
		Error  interface{}
	}
)

var ErrUnknownType = errors.New("unknown type")

var ErrBadInput = errors.New("bad input")

const (
	Subscribe      RequestType  = "mining.subscribe"
	Authorize      RequestType  = "mining.authorize"
	Submit         RequestType  = "mining.submit"
	Notify         ResponseType = "mining.notify"
	SubscribeReply ResponseType = "mining.subscribe.reply"
	Version        ResponseType = "client.get_version"
	Difficulty     ResponseType = "mining.set_difficulty"
	Extranonce     ResponseType = "mining.set_extranonce"
	Reconnect      ResponseType = "client.reconnect"
	General        ResponseType = "general"
)

func marshalRequest(request RawRPC, params interface{}) ([]byte, error) {
	result, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	request.Params = (*json.RawMessage)(&result)
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (r RequestBase) Type() RequestType {
	return r.Method
}

func (r RequestSubscribe) MarshalJSON() ([]byte, error) {
	return marshalRequest(RawRPC{
		ID:     r.ID,
		Method: Subscribe,
	}, make([]int, 0))
}

func (r RequestAuthorize) MarshalJSON() ([]byte, error) {
	return marshalRequest(RawRPC{
		ID:     r.ID,
		Method: Authorize,
	}, []string{r.Username, r.Password})
}

func (r RequestSubmit) MarshalJSON() ([]byte, error) {
	return marshalRequest(RawRPC{
		ID:     r.ID,
		Method: Submit,
	}, []interface{}{r.Worker, r.Job, r.NTime, r.NoncePart2})
}

func Parse(data []byte) (Request, error) {
	var raw RawRPC
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	base := RequestBase{
		ID:     raw.ID,
		Method: raw.Method,
	}

	switch RequestType(raw.Method) {
	case Subscribe:
		return RequestSubscribe{
			RequestBase: base,
			Params:      nil,
		}, nil

	case Authorize:
		var params [2]string
		if err := json.Unmarshal(*raw.Params, &params); err != nil {
			log.Println(err)
			return nil, errwrap.Wrapf("error decoding auth params: {{err}}", ErrBadInput)
		}

		return RequestAuthorize{
			RequestBase: base,
			Username:    params[0],
			Password:    params[1],
		}, nil

	case Submit:
		var params [5]string
		if err := json.Unmarshal(*raw.Params, &params); err != nil {
			return nil, errwrap.Wrapf("error decoding submit params: {{err}}", ErrBadInput)
		}

		ntime, err := HexToUint32(params[2])
		if err != nil {
			return nil, ErrBadInput
		}

		nonce, err := HexToUint128(params[3])
		if err != nil {
			return nil, ErrBadInput
		}

		solution, err := readHex(params[4], 1347)
		if err != nil {
			return nil, ErrBadInput
		}
		solution = solution[3:]

		return RequestSubmit{
			RequestBase: base,
			Worker:      params[0],
			Job:         params[1],
			NTime:       ntime,
			NoncePart2:  nonce,
			Solution:    solution,
		}, nil

	default:
		return nil, ErrUnknownType
	}
}

func (r ResponseSubscribeReply) MashalJSON() ([]byte, error) {
	return json.Marshal(ResponseGeneral{
		ID:     r.ID,
		Result: []interface{}{r.Session, ToHex(r.NoncePart1)},
	})
}

func (r ResponseSubscribeReply) Type() ResponseType {
	return SubscribeReply
}

func (r ResponseNotify) MarshalJSON() ([]byte, error) {
	return marshalRequest(RawRPC{
		ID:     nil,
		Method: RequestType(Notify),
	}, []interface{}{
		r.Job,
		ToHex(r.Version),
		ToHex(r.HashPrevBlock),
		ToHex(r.HashMerkleRoot),
		ToHex(r.HashReserved),
		ToHex(r.NTime),
		ToHex(r.NBits),
		r.CleanJobs,
	})
}

func (r ResponseNotify) Type() ResponseType {
	return Notify
}

func (r ResponseSetDifficulty) MarshalJSON() ([]byte, error) {
	return marshalRequest(RawRPC{
		ID:     nil,
		Method: ResponseType(SetDifficulty),
	}, []interface{}{
		ToHex(r.Target),
	})
}

func (r ResponseSetDifficulty) Type() ResponseType {
	return SetDifficulty
}

func (r ResponseGeneral) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":     r.ID,
		"result": r.Result,
		"error":  r.Error,
	})
}

func (r ResponseGeneral) Type() ResponseType {
	return General
}

func (uint256 Uint256) ToInteger() *big.Int {
	x := big.NewInt(0)
	return x.SetBytes(uint256[:])
}
