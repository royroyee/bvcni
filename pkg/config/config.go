package config

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"os"
)

const cniConfigPath = "/etc/cni/net.d/00-bvcni.conf"

var cniConfTemplate = `{
  "cniVersion": "0.3.1",
  "name": "bvcni",
  "type": "bvcni",
  "podcidr": "%s"
}`

type CNIConfig struct {
	CNIVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	PodCidr    string `json:"podcidr"`
}

func InitCNIPluginConfigFile(node *v1.Node) error {

	// Check Node's PodCIDR
	if node.Spec.PodCIDR == "" {
		return errors.Errorf("node : %s is not set podCIDR ", node.Name)
	}

	fd, err := os.OpenFile(cniConfigPath,
		os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.ModeAppend|os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "open cni config file error")
	}

	defer fd.Close()

	if _, err = fd.Write([]byte(fmt.Sprintf(cniConfTemplate, node.Spec.PodCIDR))); err != nil {
		return errors.Wrap(err, "write cni config file error")
	}

	return nil
}

func LoadCNIConfig(stdinData []byte) (*CNIConfig, error) {
	var config CNIConfig

	if err := json.Unmarshal(stdinData, &config); err != nil {
		log.Debugf("LoadCNIConfig error: %s", err.Error())
		return nil, errors.Wrap(err, "json Unmarshal error")
	}

	return &config, nil
}
