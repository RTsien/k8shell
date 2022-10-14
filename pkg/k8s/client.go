package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/rtsien/k8shell/pkg/utils"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// Client wrapper k8s client and namespace
type Client struct {
	kubernetes.Interface
	config *rest.Config
}

// NewClient create a new client
func NewClient(kubeconfig string) (*Client, error) {
	k8sConfig, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))
	if err != nil {
		return nil, err
	}
	config, err := k8sConfig.ClientConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create kubeconfig[%s], err[%s].\n", kubeconfig, err.Error())
		return nil, err
	}
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{Interface: cli, config: config}, err
}

// CopyFileToPod copy client file to pod
func (c *Client) CopyFileToPod(pod, container, namespace string, file io.Reader, dstPath string) error {
	dstDir := path.Dir(dstPath)
	execCmd := fmt.Sprintf("mkdir -p %s && cd %s && tar x", dstDir, dstDir)

	cmd := []string{
		"sh",
		"-c",
		execCmd,
	}
	fmt.Printf("exec command: %s\n", cmd)
	req := c.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).
		Namespace(namespace).SubResource("exec")

	req.VersionedParams(
		&corev1.PodExecOptions{
			Command:   cmd,
			Container: container,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		},
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  file,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	fmt.Printf("copy file to pod[%s] container[%s], stdout[%s] stderr[%s]\n",
		pod, container, stdout.String(), stderr.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "copy file to pod[%s] container[%s] failed, err[%s].\n", pod, container, err.Error())
		return err
	}
	return nil
}

// GetPod specified pod in specified namespace.
func (c *Client) GetPod(ctx context.Context, name, namespace string) (*corev1.Pod, error) {
	opt := metav1.GetOptions{}
	return c.CoreV1().Pods(namespace).Get(ctx, name, opt)
}

// Exec exec into a pod
func (c *Client) Exec(cmd []string, ptyHandler PtyHandler, namespace, podName, containerName string) error {
	defer func() {
		ptyHandler.Done()
	}()

	req := c.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
		Stdin:     !(ptyHandler.Stdin() == nil),
		Stdout:    !(ptyHandler.Stdout() == nil),
		Stderr:    !(ptyHandler.Stderr() == nil),
		TTY:       ptyHandler.Tty(),
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return err
	}
	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:             ptyHandler.Stdin(),
		Stdout:            ptyHandler.Stdout(),
		Stderr:            ptyHandler.Stderr(),
		TerminalSizeQueue: ptyHandler,
		Tty:               ptyHandler.Tty(),
	})
	return err
}

// Logs get logs of specified pod in specified namespace.
func (c *Client) Logs(name, namespace string, opts *corev1.PodLogOptions) *rest.Request {
	return c.CoreV1().Pods(namespace).GetLogs(name, opts)
}

// LogStreamLine get logs of specified pod in specified namespace and copy to writer.
func (c *Client) LogStreamLine(ctx context.Context, name, namespace string, opts *corev1.PodLogOptions, writer io.Writer) error {
	req := c.Logs(name, namespace, opts)
	r, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	bufReader := bufio.NewReaderSize(r, 256)
	// bufReader := bufio.NewReader(r)
	for {
		line, _, err := bufReader.ReadLine()
		// line = []byte(fmt.Sprintf("%s", string(line)))
		line = utils.ToValidUTF8(line, []byte(""))
		if err != nil {
			if err == io.EOF {
				_, err = writer.Write(line)
			}
			return err
		}
		// line = append(line, []byte("\r\n")...)
		// line = append(bytes.Trim(line, " "), []byte("\r\n")...)
		_, err = writer.Write(line)
		if err != nil {
			return err
		}
	}
}
