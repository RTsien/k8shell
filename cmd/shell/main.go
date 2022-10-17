package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"

	"github.com/rtsien/k8shell/pkg/k8s"
	"github.com/rtsien/k8shell/pkg/utils"
	ws "github.com/rtsien/k8shell/pkg/websocket"
)

var (
	addr                = flag.String("addr", ":8080", "http service address")
	defaultCmd          = []string{"/bin/bash"}
	defaultTail   int64 = 200
	defaultFollow       = true
)

var (
	kubeconfig map[string]*k8s.Client
	once       sync.Once
	rwm        sync.RWMutex
)

func serveTerminal(w http.ResponseWriter, r *http.Request) {
	// auth
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "./frontend/terminal.html")
}

func serveLogs(w http.ResponseWriter, r *http.Request) {
	// auth
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "./frontend/logs.html")
}

func serveWsTerminal(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	cluster := pathParams["cluster"]
	namespace := pathParams["namespace"]
	podName := pathParams["pod"]
	containerName := pathParams["container"]
	cmd := r.URL.Query()["cmd"]
	if len(cmd) == 0 {
		cmd = defaultCmd
	}
	log.Printf("exec cluster:%s, namespace: %s, pod: %s, container: %s, cmd: %v\n",
		cluster, namespace, podName, containerName, cmd)

	pty, err := ws.NewTerminalSession(w, r, nil)
	if err != nil {
		log.Printf("get pty failed: %v\n", err)
		return
	}
	defer func() {
		log.Println("close session.")
		pty.Done()
		_ = pty.Close()
	}()

	client := getClient(cluster)
	if client == nil {
		log.Println("get kubernetes client failed")
		return
	}
	pod, err := client.GetPod(context.Background(), podName, namespace)
	if err != nil {
		log.Printf("get kubernetes client failed: %v\n", err)
		return
	}
	ok, err := k8s.ValidatePod(pod, containerName)
	if !ok {
		msg := fmt.Sprintf("Validate pod error! err: %v", err)
		log.Println(msg)
		_, _ = pty.Write([]byte(msg))
		return
	}
	err = client.Exec(cmd, pty, namespace, podName, containerName)
	if err != nil {
		msg := fmt.Sprintf("Exec to pod error! err: %v", err)
		log.Println(msg)
		_, _ = pty.Write([]byte(msg))
	}
	return
}

func serveWsLogs(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	cluster := pathParams["cluster"]
	namespace := pathParams["namespace"]
	podName := pathParams["pod"]
	containerName := pathParams["container"]
	tailLine := defaultTail
	if r.URL.Query().Has("tail") {
		tailLine, _ = utils.StringToInt64(r.URL.Query().Get("tail"))
	}
	follow := defaultFollow
	if r.URL.Query().Has("follow") {
		follow, _ = utils.StringToBool(r.URL.Query().Get("follow"))
	}
	log.Printf("exec cluster:%s, namespace: %s, pod: %s, container: %s\n",
		cluster, namespace, podName, containerName)
	writer, err := k8s.NewWsLogger(w, r, nil)
	if err != nil {
		log.Printf("get writer failed: %v\n", err)
		return
	}
	defer func() {
		log.Println("close session.")
		_ = writer.Close()
	}()

	client := getClient(cluster)
	if client == nil {
		log.Println("get kubernetes client failed")
		return
	}
	pod, err := client.GetPod(context.Background(), podName, namespace)
	if err != nil {
		log.Printf("get kubernetes client failed: %v\n", err)
		return
	}
	ok, err := k8s.ValidatePod(pod, containerName)
	if !ok {
		msg := fmt.Sprintf("Validate pod error! err: %v", err)
		log.Println(msg)
		_, _ = writer.Write([]byte(msg))
		_ = writer.Close()
		return
	}

	opt := corev1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
		TailLines: &tailLine,
	}

	err = client.LogStreamLine(context.Background(), podName, namespace, &opt, writer)
	if err != nil {
		msg := fmt.Sprintf("log err: %v", err)
		log.Println(msg)
		_, _ = writer.Write([]byte(msg))
		_ = writer.Close()
	}
	return
}

func getClient(cluster string) *k8s.Client {
	rwm.RLock()
	defer rwm.RUnlock()
	return kubeconfig[cluster]
}

func updateKubeconfig() {
	once.Do(func() {
		kubeconfig = make(map[string]*k8s.Client)
	})
	_ = filepath.WalkDir("./kubeconfig/", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if k, err := os.ReadFile(path); err != nil {
			return err
		} else {
			rwm.Lock()
			defer rwm.Unlock()
			cli, err := k8s.NewClient(string(k))
			if err != nil {
				return err
			}

			kubeconfig[d.Name()] = cli
		}

		log.Println("kubeconfig:", d.Name())
		return nil
	})
}

func main() {
	s := gocron.NewScheduler(time.Local).StartImmediately()
	_, _ = s.Every(1).Minute().Do(updateKubeconfig)
	s.StartAsync()

	router := mux.NewRouter()
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./frontend/"))))
	// http://127.0.0.1:8090/terminal?cluster=abc&namespace=default&pod=nginx-0&container=nginx
	// http://127.0.0.1:8090/terminal?cluster=abc&namespace=default&pod=nginx-0&container=nginx&cmd=/bin/bash
	router.HandleFunc("/terminal", serveTerminal)
	router.HandleFunc("/ws/{cluster}/{namespace}/{pod}/{container}/webshell", serveWsTerminal)
	// http://127.0.0.1:8090/logs?cluster=abc&namespace=default&pod=nginx-0&container=nginx
	// http://127.0.0.1:8090/logs?cluster=abc&namespace=default&pod=nginx-0&container=nginx&tail=200&follow=true
	router.HandleFunc("/logs", serveLogs)
	router.HandleFunc("/ws/{cluster}/{namespace}/{pod}/{container}/logs", serveWsLogs)
	log.Fatal(http.ListenAndServe(*addr, router))
}
