package main

import (
	"flag"
	"fmt"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"go-image-process/internal/conf"
	"go-image-process/internal/logging"
	"gopkg.in/yaml.v3"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// go build -ldflags "-X main.Version=x.y.z -X main.Env=dev"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagConf is the config flag.
	flagConf    string
	Env         string
	logFileName string

	id, _ = os.Hostname()

	httpServerPort string
)

func init() {
	flag.StringVar(&flagConf, "conf", "../configs", "config path, eg: -conf config.yaml")
	flag.StringVar(&logFileName, "log", "go-image-process", "set log file name")
	flag.StringVar(&httpServerPort, "http.port", "", "set the http serve port")

}

func newApp(hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Server(
			hs,
		),
	)
}

func main() {

	flag.Parse()
	logger, clean := logging.NewLogger(logFileName, Env, 3)
	defer clean()

	log.SetLogger(logger)

	c := config.New(
		config.WithSource(
			file.NewSource(flagConf),
		),
		config.WithDecoder(func(kv *config.KeyValue, v map[string]interface{}) error {
			return yaml.Unmarshal(kv.Value, v)
		}),
	)

	if c == nil {
		panic("init config wrong")
	}

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}
	fmt.Println(bc.String())
	if len(httpServerPort) > 0 {
		bc.GetServer().GetHttp().Addr = fmt.Sprintf("0.0.0.0:%s", httpServerPort)
	}

	if bc.GetImage() == nil {
		bc.Image = &conf.Image{Quality: 100}
	}

	app, cleanup, err := initApp(&bc)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
