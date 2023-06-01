package logs

import (
	"context"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Options struct {
	Pod       string
	Container string
	Namespace string
}

func Print(ctx context.Context, config *rest.Config, options Options) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	req := clientset.CoreV1().Pods(options.Namespace).GetLogs(options.Pod, &corev1.PodLogOptions{
		Container: options.Container,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to get response: %w", err)
	}
	defer stream.Close()

	for {
		const bufSize = 2000
		buf := make([]byte, bufSize)
		numBytes, err := stream.Read(buf)
		if numBytes == 0 {
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		message := string(buf[:numBytes])
		fmt.Print(message)
	}

	return nil
}
