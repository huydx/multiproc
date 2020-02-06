package main

import (
	"github.com/huydx/multiproc"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
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
	mt := multiproc.New(&cfg)
	_ = mt.Start()
}
