package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	v1 "k8s.io/kubernetes/pkg/api/v1"
	kubeletconfig "k8s.io/kubernetes/pkg/kubelet/apis/config"
	kubeletconfigscheme "k8s.io/kubernetes/pkg/kubelet/apis/config/scheme"
)

func main() {
	// uses the current context in kubeconfig
	// path-to-kubeconfig -- for example, /root/.kube/config
	config, _ := clientcmd.BuildConfigFromFlags("", "/root/.kube/config")
	// creates the clientset
	clientset, _ := kubernetes.NewForConfig(config)
	// access the API to list pods
	nodes, _ := clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	fmt.Printf("%v", nodes)
	//nodes, _ := clientset.CoreV1().ConfigMaps("").List(context.TODO(), v1.ListOptions{})
	clientset.REST()
	//fmt.Printf("%v", nodes.Items)

	endpoint := fmt.Sprintf("http://localhost:8001/api/v1/nodes/kind-control-plane/proxy/configz")
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", endpoint, nil)
	req.Header.Add("Accept", "application/json")

	var resp *http.Response
	err = wait.PollImmediate(1, 5, func() (bool, error) {
		resp, err = client.Do(req)
		if err != nil {
			fmt.Printf("Failed to get /configz, retrying. Error: %v", err)
			return false, nil
		}
		if resp.StatusCode != 200 {
			fmt.Printf("/configz response status not 200, retrying. Response was: %+v", resp)
			return false, nil
		}

		return true, nil
	})
	conf, err := decodeConfigz(resp)
	if err == nil {
		fmt.Printf("our topology manager policy is %v\n", conf.TopologyManagerPolicy)
	}

}

// Decodes the http response from /configz and returns a kubeletconfig.KubeletConfiguration (internal type).
func decodeConfigz(resp *http.Response) (*kubeletconfig.KubeletConfiguration, error) {
	// This hack because /configz reports the following structure:
	// {"kubeletconfig": {the JSON representation of kubeletconfigv1beta1.KubeletConfiguration}}
	type configzWrapper struct {
		ComponentConfig kubeletconfigv1beta1.KubeletConfiguration `json:"kubeletconfig"`
	}

	configz := configzWrapper{}
	kubeCfg := kubeletconfig.KubeletConfiguration{}

	contentsBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(contentsBytes, &configz)
	if err != nil {
		return nil, err
	}

	scheme, _, err := kubeletconfigscheme.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	err = scheme.Convert(&configz.ComponentConfig, &kubeCfg, nil)
	if err != nil {
		return nil, err
	}

	return &kubeCfg, nil
}
