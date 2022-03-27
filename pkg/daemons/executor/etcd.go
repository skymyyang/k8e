package executor

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	daemonconfig "github.com/xiaods/k8e/pkg/daemons/config"
	"github.com/xiaods/k8e/pkg/version"
	"go.etcd.io/etcd/embed"
	"go.etcd.io/etcd/etcdserver"
)

type Embedded struct {
	nodeConfig *daemonconfig.Node
}

func (e *Embedded) ETCD(ctx context.Context, args ETCDConfig, extraArgs []string) error {
	configFile, err := args.ToConfigFile(extraArgs)
	if err != nil {
		return err
	}
	cfg, err := embed.ConfigFromFile(configFile)
	if err != nil {
		return err
	}

	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		return err
	}

	go func() {
		select {
		case err := <-etcd.Server.ErrNotify():
			if strings.Contains(err.Error(), etcdserver.ErrMemberRemoved.Error()) {
				tombstoneFile := filepath.Join(args.DataDir, "tombstone")
				if err := ioutil.WriteFile(tombstoneFile, []byte{}, 0600); err != nil {
					logrus.Fatalf("failed to write tombstone file to %s", tombstoneFile)
				}
				logrus.Infof("this node has been removed from the cluster please restart %s to rejoin the cluster", version.Program)
				return
			}

		case <-ctx.Done():
			logrus.Infof("stopping etcd")
			etcd.Close()
		case <-etcd.Server.StopNotify():
			logrus.Fatalf("etcd stopped")
		case err := <-etcd.Err():
			logrus.Fatalf("etcd exited: %v", err)
		}
	}()
	return nil
}
