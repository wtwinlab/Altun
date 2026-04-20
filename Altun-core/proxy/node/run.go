package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	pbv "github.com/wtwinlab/Altun/layers/proxy/proto"
	"google.golang.org/grpc"
)

func main() {

	configFile := flag.String("config", "", "The path to the config file")
	flag.Parse()

	var conf *Options
	fmt.Println(*configFile)
	err := ReadConfig(&conf, *configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	svr := NewNode(conf.SelfID) //&conf, *configFile)
	if err != nil {
		//panic(err)
		fmt.Printf("%v", err)

	}
	pbv.RegisterProxyServer(s, svr)
	lis, err := net.Listen("tcp", conf.ServerNodeAddr)

	if err != nil {
		//panic(err)
		fmt.Printf("%v", err)
	} else {
		fmt.Println(" Node listen address: " + conf.ServerNodeAddr)
	}

	go func() {
		fmt.Printf(" Node Serving gRPC: %s", conf.ServerNodeAddr)
		s.Serve(lis)
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	sig := <-ch
	fmt.Printf("Received signal %v, quiting gracefully", sig)
}

func ReadConfig(opts interface{}, configfile string) (err error) {
	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return err
	}
	if configfile != "" {
		viper.SetConfigName(configfile)
		viper.SetConfigType("toml")
		viper.AddConfigPath("/etc/altun/")
		viper.AddConfigPath("$HOME/.altun")
		viper.AddConfigPath(".")
		viper.AddConfigPath("../")

	} else {
		viper.SetConfigName("altun")
	}

	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		return err
	}
	err = viper.Unmarshal(opts)
	if err != nil {
		return err
	}

	return nil
}
