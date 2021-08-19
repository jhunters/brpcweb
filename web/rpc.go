/*
 * @Author: Malin Xie
 * @Description:
 * @Date: 2021-07-26 14:40:41
 */
package web

import (
	"encoding/json"
	"fmt"
	"time"

	baidurpc "github.com/baidu-golang/pbrpc"
	"github.com/golang/protobuf/proto"
)

const (
	Default_Timeout = 10 * time.Second
)

type RpcOptions struct {
	Name    string    `json:"name,omitempty"`
	Host    string    `json:"host,omitempty"`
	Port    int       `json:"port,omitempty"`
	Date    time.Time `json:"date,omitempty"`
	DateStr string    `json:"datestr,omitempty"`
}

func marshalRpcOptions(options *RpcOptions) ([]byte, error) {
	return json.Marshal(options)
}

func unmarshalRpcOptions(data []byte) (*RpcOptions, error) {
	options := &RpcOptions{}
	err := json.Unmarshal(data, options)
	if err != nil {
		return nil, err
	}
	return options, nil
}

// loadRpcRequestStatus
func loadRpcRequestStatus(host string, port int, sName, mName string) (*baidurpc.QpsData, error) {

	serviceName := baidurpc.RPC_STATUS_SERVICENAME
	methodName := "QpsDataStatus"

	parameterOut := &baidurpc.QpsData{}
	parameterIn := &baidurpc.RPCMethod{Service: sName, Method: mName}
	err := sendRpc(host, port, serviceName, methodName, parameterIn, parameterOut)

	return parameterOut, err
}

// loadRpcStatus
func loadRpcStatus(host string, port int) (*baidurpc.RPCStatus, error) {

	serviceName := baidurpc.RPC_STATUS_SERVICENAME
	methodName := "Status"

	parameterOut := &baidurpc.RPCStatus{}
	err := sendRpc(host, port, serviceName, methodName, nil, parameterOut)

	return parameterOut, err
}

func sendRpc(host string, port int, serviceName, methodName string, parameterIn, parameterOut proto.Message) error {
	url := baidurpc.URL{}
	url.SetHost(&host).SetPort(&port)

	timeout := Default_Timeout
	connection, err := baidurpc.NewTCPConnection(url, &timeout)
	if err != nil {
		return err
	}
	defer connection.Close()

	// create client
	rpcClient, err := baidurpc.NewRpcCient(connection)
	if err != nil {
		return err
	}
	defer rpcClient.Close()

	rpcInvocation := baidurpc.NewRpcInvocation(&serviceName, &methodName)
	if parameterIn != nil {
		rpcInvocation.SetParameterIn(parameterIn)
	}

	rpcDataPackage, err := rpcClient.SendRpcRequest(rpcInvocation, parameterOut)
	if int(*rpcDataPackage.Meta.GetResponse().ErrorCode) == baidurpc.ST_SERVICE_NOTFOUND {
		return fmt.Errorf("remote server not support this feature,  please upgrade version")
	}

	return err
}
