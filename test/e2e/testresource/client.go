package testresource

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
)

type Client struct {
	ClientSet     *kubernetes.Clientset
	DynamicClient dynamic.Interface
}

func NewClient() (*Client, error) {
	client := new(Client)
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); err != nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		if client.ClientSet, err = kubernetes.NewForConfig(config); err != nil {
			return nil, err
		}
		if client.DynamicClient, err = dynamic.NewForConfig(config); err != nil {
			return nil, err
		}
		return client, nil
	} else {
		filePath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err := clientcmd.BuildConfigFromFlags("", filePath)
		if err != nil {
			return nil, err
		}
		if client.ClientSet, err = kubernetes.NewForConfig(config); err != nil {
			return nil, err
		}
		if client.DynamicClient, err = dynamic.NewForConfig(config); err != nil {
			return nil, err
		}
		return client, nil
	}
}
