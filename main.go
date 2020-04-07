package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/rancher/kine/pkg/client"
	kineep "github.com/rancher/kine/pkg/endpoint"
	"go.etcd.io/etcd/clientv3"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	mode     string
	endpoint string
	db       string
)

func main() {
	app := cli.NewApp()
	app.Name = "migrator"
	app.Description = "Tool to migrate etcd to dqlite"
	app.UsageText = "copy etcd data to kine"
	app.UsageText = "migrator --mode [backup|restore] --endpoint [etcd or kine endpoint] --db-dir [dir to store entries]"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name: "endpoint",
			// Value:       "http://127.0.0.1:12379",
			Value:       "unix:///var/snap/microk8s/1314/var/kubernetes/backend/kine.sock",
			Destination: &endpoint,
		},
		cli.StringFlag{
			Name:  "mode",
			Value: "backup",
			//Value:       "restore",
			Destination: &mode,
		},
		cli.StringFlag{
			Name:        "db-dir",
			Value:       "db",
			Destination: &db,
		},
		cli.BoolFlag{Name: "debug"},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func backup(ep string, dir string) error {
	ctx := context.Background()
	cli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{endpoint},
	})
	check(err)
	defer cli.Close()
	resp, err := cli.Get(ctx, "", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	check(err)

	err = os.Mkdir(db, 0700)
	check(err)

	for i, ev := range resp.Kvs {
		logrus.Debugf("%d) %s\n", i, ev.Key)
		var keyfile = db + "/" + strconv.Itoa(i) + ".key"
		var datafile = db + "/" + strconv.Itoa(i) + ".data"
		keyf, err := os.Create(keyfile)
		check(err)
		defer keyf.Close()
		dataf, err := os.Create(datafile)
		check(err)
		defer dataf.Close()
		keyf.Write(ev.Key)
		dataf.Write(ev.Value)
	}
	return nil
}

func restore(ep string, dir string) error {
	ctx := context.Background()
	etcdcfg := kineep.ETCDConfig{
		Endpoints:   []string{endpoint},
		LeaderElect: false,
	}
	c, err := client.New(etcdcfg)
	check(err)
	defer c.Close()

	var i = 0
	for true {
		var keyfile = db + "/" + strconv.Itoa(i) + ".key"
		var datafile = db + "/" + strconv.Itoa(i) + ".data"
		_, err := os.Stat(keyfile)
		if os.IsNotExist(err) {
			fmt.Printf("Processed %d entries", i)
			break
		}

		keybytes, err := ioutil.ReadFile(keyfile)
		check(err)
		key := string(keybytes)
		databytes, err := ioutil.ReadFile(datafile)
		check(err)

		logrus.Debugf("%d) %s", i, key)
		err = c.Create(ctx, key, databytes)
		if err != nil {
			if err.Error() == "key exists" {
				err = c.Put(ctx, key, databytes)
				check(err)
			} else {
				panic(err)
			}
		}
		i++
	}
	return nil
}

func run(c *cli.Context) {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	logrus.Infof("mode: %s, endpoint: %s, dir: %s", mode, endpoint, db)
	if mode == "backup" {
		err := backup(endpoint, db)
		if err != nil {
			logrus.Fatal(err)
		}
	} else if mode == "restore" {
		err := restore(endpoint, db)
		if err != nil {
			logrus.Fatal(err)
		}
	} else {
		logrus.Fatal("Unknown mode")
		return
	}

	return
}
