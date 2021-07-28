/*
 * web entrance file. process all http reqeust and response action
 * @Author: Malin Xie
 * @Description:
 * @Date: 2021-07-26 12:34:45
 */
package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	baidurpc "github.com/baidu-golang/pbrpc"
	"github.com/gin-gonic/gin"
	bolt "go.etcd.io/bbolt"
)

const (
	Default_PrefixPath = "/___"
	BlotFile           = "server.data"
	Bucket_Name        = "baidurpc_bucket"

	ErrorCode = -1
)

var (
	Use_Embed_Mode = true

	//go:embed html/*.html
	webdir embed.FS

	//go:embed static/* static/fonts/*
	content embed.FS
)

// ResponseData standard web response struct to front end.
type ResponseData struct {
	ErrNo   int         `json:"errno"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var emptyResponseData = ResponseData{}

// WebModule web module entrance
type WebModule struct {
	listenAddr string
	prefixPath string
	db         *bolt.DB
	listener   net.Listener
	webdir     string
}

type RpcInfoList []RpcOptions

func (s RpcInfoList) Len() int           { return len(s) }
func (s RpcInfoList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s RpcInfoList) Less(i, j int) bool { return s[i].Date.After(s[j].Date) }

type IntSort []int64

func (s IntSort) Len() int           { return len(s) }
func (s IntSort) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s IntSort) Less(i, j int) bool { return s[i] <= s[j] }

type RPCMethodList []*baidurpc.RPCMethod

func (s RPCMethodList) Len() int           { return len(s) }
func (s RPCMethodList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s RPCMethodList) Less(i, j int) bool { return strings.Compare(s[i].Service, s[j].Service) > 0 }

// NewWebModule create a new web module.
func NewWebModule(listenAddr string, prefixPath string, datadir string) (*WebModule, error) {

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	return NewWebModuleWithListener(listener, prefixPath, datadir)
}

// NewWebModule create a new web module.
func NewWebModuleWithListener(l net.Listener, prefixPath string, datadir string) (*WebModule, error) {

	mime.AddExtensionType(".ttf", "font/ttf")
	mime.AddExtensionType(".woff", "font/woff")

	if len(prefixPath) == 0 {
		prefixPath = Default_PrefixPath
	}
	// mkdir
	os.MkdirAll(datadir, 0666)

	blotFile := datadir + "/" + BlotFile
	db, err := bolt.Open(blotFile, 0666, nil)
	if err != nil {
		return nil, err
	}

	return &WebModule{l.Addr().String(), prefixPath, db, l, "web"}, nil
}

// AddRPCServer
func (wm *WebModule) AddRPCServer(name string, host string, port int) error {

	options := &RpcOptions{Name: name, Host: host, Port: port, Date: time.Now()}
	v, err := marshalRpcOptions(options)
	if err != nil {
		return err
	}

	return putData(wm.db, name, Bucket_Name, v, false)
}

// StartWeb
func (wm *WebModule) StartWeb() *http.Server {
	router := gin.Default()

	wm.servStaticFiles(router)
	wm.servHtmlFiles(router)

	wm.listRpcs(router)
	wm.addRpc(router)
	wm.updateRpc(router)
	wm.deleteRpc(router)
	wm.loadRpcStatus(router)
	wm.loadRpcRequestStatus(router)

	srv := &http.Server{
		Handler: router,
	}
	go func() {
		// service connections
		if err := srv.Serve(wm.listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Printf("brpc web server started at %s", wm.listener.Addr().String())
	return srv
}

// StartWebAndBlock
func (wm *WebModule) StartWebAndBlock() {
	srv := wm.StartWeb()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of  seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown error:", err)
	}
}

// servService service defined here
func (wm *WebModule) listRpcs(router *gin.Engine) {
	// get rpc info list
	router.GET(wm.getPath("/rpc"), func(c *gin.Context) {
		withJsonHeader(c)

		// pagination
		page := c.Query("page")
		pageSize := c.Query("pagesize")
		ipage, err := strconv.Atoi(page)
		if err != nil {
			ipage = 1
		}

		ipageSize, err := strconv.Atoi(pageSize)
		if err != nil {
			ipageSize = 8
		}

		data, count, err := LoadData(wm.db, Bucket_Name, ipage, ipageSize)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
		}

		rpcInfos := make([]RpcOptions, len(data))
		var i int = 0
		for _, v := range data {
			options, err := unmarshalRpcOptions(v)
			if err != nil {
				c.JSON(http.StatusOK, *errorResponseData(err.Error()))
				return
			}
			rpcInfos[i] = *options
			rpcInfos[i].DateStr = rpcInfos[i].Date.Format("2006-01-02 15:04:05")
			i++
		}
		sort.Sort(RpcInfoList(rpcInfos))

		result := map[string]interface{}{}
		result["data"] = rpcInfos
		result["size"] = count

		c.JSON(http.StatusOK, *successResponseData(result))
	})
}

// addRpc add a new rpc info
func (wm *WebModule) addRpc(router *gin.Engine) {
	// add rpc info
	router.POST(wm.getPath("/rpc"), func(c *gin.Context) {
		body, _ := ioutil.ReadAll(c.Request.Body)
		if body != nil {
			data, err := unmarshalRpcOptions(body)
			if err != nil {
				c.JSON(http.StatusOK, errorResponseData(err.Error()))
				return
			}
			if data.Port <= 0 {
				c.JSON(http.StatusOK, errorResponseData(fmt.Sprintf("invalid port %d", data.Port)))
				return
			}
			if len(data.Host) == 0 {
				c.JSON(http.StatusOK, errorResponseData(fmt.Sprintf("invalid host name '%s'", data.Host)))
				return
			}
			data.Date = time.Now()

			value, err := marshalRpcOptions(data)
			if err != nil {
				c.JSON(http.StatusOK, errorResponseData(err.Error()))
				return
			}
			err = putData(wm.db, data.Name, Bucket_Name, value, true)
			if err != nil {
				c.JSON(http.StatusOK, errorResponseData(err.Error()))
			} else {
				c.JSON(http.StatusOK, emptyResponseData)
			}
			return
		}

		c.JSON(http.StatusOK, ResponseData{ErrNo: -1, Message: "invalid: empty post data"})
	})
}

// updateRpc update rpc info
func (wm *WebModule) updateRpc(router *gin.Engine) {
	// add rpc info
	router.PUT(wm.getPath("/rpc/:name"), func(c *gin.Context) {

		name := c.Param("name")
		host := c.Query("host")
		if len(host) == 0 {
			c.JSON(http.StatusOK, errorResponseData(fmt.Sprintf("invalid host name '%s'", host)))
			return
		}
		port := c.Query("port")

		iport, err := strconv.Atoi(port)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}
		options := &RpcOptions{Name: name, Host: host, Port: iport, Date: time.Now()}
		value, err := json.Marshal(options)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}
		err = putData(wm.db, name, Bucket_Name, value, false)

		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}
		c.JSON(http.StatusOK, emptyResponseData)
	})
}

// deleteRpc delete rpc info
func (wm *WebModule) deleteRpc(router *gin.Engine) {
	// add rpc info
	router.DELETE(wm.getPath("/rpc/:name"), func(c *gin.Context) {

		name := c.Param("name")

		err := deleteData(wm.db, name, Bucket_Name)

		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}
		c.JSON(http.StatusOK, emptyResponseData)
	})
}

// loadRpcStatus load rpc info
func (wm *WebModule) loadRpcStatus(router *gin.Engine) {
	// add rpc info
	router.GET(wm.getPath("/rpc/:name/status"), func(c *gin.Context) {

		name := c.Param("name")

		data, err := getData(wm.db, name, Bucket_Name)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}
		if data == nil {
			c.JSON(http.StatusOK, errorResponseData(fmt.Sprintf("no key found '%s'", name)))
			return
		}

		options, err := unmarshalRpcOptions(data)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}

		status, err := loadRpcStatus(options.Host, options.Port)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}

		host := status.Host
		if len(host) == 0 {
			host = options.Host
		}

		port := status.Port
		if port == 0 {
			port = int32(options.Port)
		}

		// sort method
		sort.Sort(RPCMethodList(status.Methods))

		result := map[string]interface{}{}
		result["name"] = name
		result["host"] = host
		result["port"] = port
		result["timeout"] = status.TimeoutSenconds
		result["data"] = status.Methods
		c.JSON(http.StatusOK, successResponseData(result))
	})
}

// loadRpcStatus load rpc info
func (wm *WebModule) loadRpcRequestStatus(router *gin.Engine) {
	// add rpc info
	router.GET(wm.getPath("/rpc/:name/qps"), func(c *gin.Context) {

		name := c.Param("name")
		serviceName := c.Query("service")
		methodName := c.Query("method")

		data, err := getData(wm.db, name, Bucket_Name)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}
		if data == nil {
			c.JSON(http.StatusOK, errorResponseData(fmt.Sprintf("no key found '%s'", name)))
			return
		}

		options, err := unmarshalRpcOptions(data)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}

		status, err := loadRpcRequestStatus(options.Host, options.Port, serviceName, methodName)
		if err != nil {
			c.JSON(http.StatusOK, errorResponseData(err.Error()))
			return
		}

		qpsinfo := status.Qpsinfo

		categoryData := make([]int64, len(qpsinfo))
		categorySData := make([]string, len(qpsinfo))
		seriesData := make([]int32, len(qpsinfo))
		// split key and value from map
		var i int = 0
		for k := range qpsinfo {
			categoryData[i] = k
			i++
		}

		sort.Sort(IntSort(categoryData))
		for i, k := range categoryData {
			seriesData[i] = qpsinfo[k]
			categorySData[i] = time.Unix(k, 0).Format("2006-01-02 15:04:05")
		}

		result := map[string]interface{}{}
		result["categoryData"] = categorySData
		result["seriesData"] = seriesData
		c.JSON(http.StatusOK, successResponseData(result))
	})
}

func (wm *WebModule) servStaticFiles(router *gin.Engine) {
	// static files
	static := WebDir{Prefix: "./web/static", EmbedPrefix: "./static", Content: content, Embbed: Use_Embed_Mode}
	router.StaticFS(wm.getPath("static"), static)

	router.HEAD("/favicon.ico", func(c *gin.Context) {
		favicon(c, static)
	})
	router.GET("/favicon.ico", func(c *gin.Context) {
		favicon(c, static)
	})
}

func (wm *WebModule) servHtmlFiles(router *gin.Engine) {
	// web template files
	webTemplate := TemplateFS{Content: webdir, Current: "./", Embbed: Use_Embed_Mode, DelimsLeft: "${{", DelimsRigth: "}}"}
	webTemplate.Parse(router, "./web", "html/*.html")

	// index page
	router.GET(wm.getPath("/"), func(c *gin.Context) {
		// process index page
		c.Header("Content-Type", "text/html")
		c.HTML(
			http.StatusOK, "index.html", map[string]interface{}{
				"Prefix": wm.prefixPath,
			},
		)

	})
}

func (wm *WebModule) getPath(path string) string {
	return wm.prefixPath + path
}

// favicon
func favicon(c *gin.Context, fs http.FileSystem) {
	c.FileFromFS("favicon.ico", fs)
}

// Close do close bblot db
func (wm *WebModule) Close() {
	wm.db.Close()
}

func withJsonHeader(c *gin.Context) {
	c.Header("Content-Type", "application/json")
}

func errorResponseData(message string) *ResponseData {
	return &ResponseData{ErrNo: ErrorCode, Message: message}
}

func successResponseData(data interface{}) *ResponseData {
	return &ResponseData{Data: data}
}
