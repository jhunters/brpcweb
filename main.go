/*
 * @Author: Malin Xie
 * @Description: main entrance for server to start
 * @Date: 2021-07-07 12:51:01
 */
package main

import (
	"flag"
	"fmt"

	"github.com/jhunters/brpcweb/web"
)

var (
	datadir = flag.String("datadir", "./data", "data file path")
	listen  = flag.String("http", ":8080", "host and port to listen. eg :8080")
)

func main() {

	flag.Parse()
	web.Use_Embed_Mode = false
	module, err := web.NewWebModule(*listen, "/", *datadir)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer module.Close()

	module.StartWebAndBlock()

}
