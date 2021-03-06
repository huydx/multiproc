package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/huydx/multiproc"

	"gopkg.in/yaml.v2"
)

func main() {
	cfg := multiproc.Config{}
	if len(os.Args) < 2 {
		log.Fatal("need config file path")
	}
	config := os.Args[1]
	bs, err := ioutil.ReadFile(config)
	if err != nil {
		log.Fatal(err)
	}
	if err := yaml.Unmarshal(bs, &cfg); err != nil {
		log.Fatal(err)
	}
	sigs := make(chan os.Signal, 1)
	var terms = []os.Signal{
		syscall.SIGABRT,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGSTOP,
	}
	signal.Notify(sigs, terms...)
	mt := multiproc.New(&cfg)
	go func() {
		sig := <-sigs
		fmt.Printf("get sig %v\n", sig)
		_ = mt.Stop()
	}()
	_ = mt.Start()

}
