/*
 * @Author: Malin Xie
 * @Description:
 * @Date: 2021-06-04 14:25:31
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	baidurpc "github.com/baidu-golang/pbrpc"
	"github.com/baidu-golang/pbrpc/nettool"
	"github.com/golang/protobuf/proto"
	"github.com/jhunters/brpcweb/web"
)

var port = flag.Int("port", 8122, "If non-empty, port this server to listen")

func init() {

	if !flag.Parsed() {
		flag.Parse()
	}
}

func main() {

	serverMeta := baidurpc.ServerMeta{}
	serverMeta.QPSExpireInSecs = 600
	rpcServer := baidurpc.NewTpcServer(&serverMeta)

	echoService := new(EchoService)

	mapping := make(map[string]string)
	mapping["Echo"] = "echo"
	rpcServer.RegisterNameWithMethodMapping("echoService", echoService, mapping)

	addr := ":" + strconv.Itoa(*port) // host and port
	var headsize uint8 = 4
	selector, err := nettool.NewCustomListenerSelectorByAddr("tcp", addr, headsize, nettool.Equal_Mode)
	if err != nil {
		fmt.Println(err)
		return
	}

	rpcServerListener, err := selector.RegisterListener(baidurpc.MAGIC_CODE) //"PRPC"
	if err != nil {
		fmt.Println(err)
		return
	}
	httpServerListener := selector.RegisterDefaultListener()

	// start customize listener
	go selector.Serve()
	module, err := web.NewWebModuleWithListener(httpServerListener, "/", "./data")
	if err != nil {
		fmt.Println(err)
		return
	}
	module.StartWeb()
	defer module.Close()

	module.AddRPCServer("本地", "localhost", *port)

	err = rpcServer.StartServer(rpcServerListener)

	if err != nil {
		baidurpc.Error(err)
		os.Exit(-1)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	// Block until a signal is received.
	fmt.Println("Press Ctrl+C or send kill sinal to exit.")
	<-c

	selector.Close()
}

type EchoService struct {
}

func Int(v int) *int {
	return &v
}

// Echo  test publish method with return type has context argument
func (rpc *EchoService) Echo(c context.Context, in *DataMessage) (*DataMessage, context.Context) {
	var ret = "hello "

	attachement := baidurpc.Attachement(c)
	fmt.Println("attachement", attachement)

	if len(*in.Name) == 0 {
		ret = ret + "veryone"
	} else {
		ret = ret + *in.Name
	}
	dm := DataMessage{}
	dm.Name = proto.String(ret)

	// bind attachment
	cc := baidurpc.BindAttachement(context.Background(), []byte("hello"))
	// bind with err
	// cc = baidurpc.BindError(cc, errors.New("manule error"))
	return &dm, cc
}

// EchoWithoutContext
func (rpc *EchoService) EchoWithoutContext(c context.Context, in *DataMessage) *DataMessage {
	dm, _ := rpc.Echo(c, in)
	return dm
}

//手工定义pb生成的代码, tag 格式 = protobuf:"type,order,req|opt|rep|packed,name=fieldname"
type DataMessage struct {
	Name *string `protobuf:"bytes,1,req,name=name" json:"name,omitempty"`
}

func (m *DataMessage) Reset()         { *m = DataMessage{} }
func (m *DataMessage) String() string { return proto.CompactTextString(m) }
func (*DataMessage) ProtoMessage()    {}

func (m *DataMessage) GetName() string {
	if m.Name != nil {
		return *m.Name
	}
	return ""
}
